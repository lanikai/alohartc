package ice

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

const (
	// Number of local candidates to buffer within channel before blocking
	localCandidateBufferLength = 8

	// Number of read packets to buffer in channel before blocking
	packetBufferLength = 16

	// Number of ready signals to buffer within channel before blocking
	readySignalBufferLength = 1
)

// RFC 8445: https://tools.ietf.org/html/rfc8445

// In the language of the above specification, this is a Full implementation of a Controlled ICE
// agent, supporting a single component of a single data stream.
type Agent struct {
	mid            string
	username       string
	localPassword  string
	remotePassword string

	localCandidates  []Candidate
	remoteCandidates []Candidate

	checklist Checklist

	// Channel for relaying local ICE candidates to the signaling layer.
	lcand chan Candidate

	// Connection for the data stream.
	dataConn  *ChannelConn
	ready     chan *ChannelConn
	readyOnce sync.Once

	agentContext context.Context
	agentCancel  context.CancelFunc
}

// NewAgentWithContext creates a new ICE agent. The agent is used to establish
// a peer connection.
func NewAgentWithContext(ctx context.Context) *Agent {
	agentContext, agentCancel := context.WithCancel(ctx)

	return &Agent{
		lcand:        make(chan Candidate, localCandidateBufferLength),
		ready:        make(chan *ChannelConn, readySignalBufferLength),
		agentContext: agentContext,
		agentCancel:  agentCancel,
	}
}

func (a *Agent) Configure(mid, username, localPassword, remotePassword string) {
	a.mid = mid
	a.username = username
	a.localPassword = localPassword
	a.remotePassword = remotePassword
}

// EstablishConnection creates a connection with a remote peer
func (a *Agent) EstablishConnection() (net.Conn, error) {
	return a.EstablishConnectionWithContext(context.Background())
}

// EstablishConnectionWithContext creates a connection with a remote agent.
// On success, returns a net.Conn object from which data can be read/written.
func (a *Agent) EstablishConnectionWithContext(ctx context.Context) (net.Conn, error) {
	if a.username == "" {
		return nil, errors.New("ICE agent not configured")
	}

	// TODO: Support multiple components
	component := 1

	bases, err := establishBases(component)
	if err != nil {
		return nil, err
	}

	// Start gathering local candidates, trickling them to the remote agent via a.lcand.
	go func() {
		err := a.gatherLocalCandidates(bases)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Begin connectivity checks.
	go func() {
		var wg sync.WaitGroup

		wg.Add(len(bases))
		for _, base := range bases {
			go func() {
				a.loop(base)
				wg.Done()
			}()
		}

		// When all base loops have terminated, agent is done
		wg.Wait()
		a.agentCancel()
	}()

	// Wait for a candidate to be selected.
	select {
	case conn := <-a.ready:
		return conn, nil
	case <-ctx.Done():
		return nil, errors.New("agent context terminated")
	}
}

func (a *Agent) ReceiveLocalCandidates() <-chan Candidate {
	return a.lcand
}

func (a *Agent) AddRemoteCandidate(c Candidate) error {
	a.remoteCandidates = append(a.remoteCandidates, c)
	// Pair new remote candidate with all existing local candidates.
	a.checklist.addCandidatePairs(a.localCandidates, []Candidate{c})
	return nil
}

func (a *Agent) addLocalCandidate(c Candidate) {
	a.localCandidates = append(a.localCandidates, c)
	// Pair new local candidate with all existing remote candidates.
	a.checklist.addCandidatePairs([]Candidate{c}, a.remoteCandidates)
	a.lcand <- c
}

// Gather local candidates. Pass candidates to lcand as they become known.
func (a *Agent) gatherLocalCandidates(bases []*Base) error {
	var wg sync.WaitGroup
	wg.Add(len(bases))
	for _, base := range bases {
		go func(base *Base) {
			log.Info("Gathering local candidates for base %s\n", base.address)
			// Host candidate for peers on the same LAN.
			hc := makeHostCandidate(a.mid, base)
			a.addLocalCandidate(hc)

			if base.address.protocol == UDP && !base.address.linkLocal {
				// Query STUN server to get a server reflexive candidate.
				mappedAddress, err := base.queryStunServer(flagStunServer)
				if err != nil {
					log.Warn("Failed to create STUN server candidate for base %s: %s\n", base.address, err)
				} else if mappedAddress == base.address {
					log.Warn("Server-reflexive address for %s is same as base\n", base.address)
				} else {
					c := makeServerReflexiveCandidate(a.mid, mappedAddress, base, flagStunServer)
					a.addLocalCandidate(c)
				}

				// TODO: TURN
			}

			wg.Done()
		}(base)
	}

	wg.Wait()
	close(a.lcand)
	return nil
}

// Done returns a channel that's closed when the agent is terminated, either
// because its parent context was terminated, or because all bases timed out,
// meaning no further peer communication will be attempted.
//
// Done is provided for use in select statements.
func (a *Agent) Done() <-chan struct{} {
	return a.agentContext.Done()
}

func (a *Agent) loop(base *Base) {
	dataIn := make(chan []byte, packetBufferLength)

	loopContext, loopCancel := context.WithCancel(a.agentContext)
	go func() {
		defer loopCancel()
		base.demuxStun(loopContext, a.handleStun, dataIn)
	}()

	Ta := time.NewTicker(50 * time.Millisecond)
	defer Ta.Stop()

	Tr := time.NewTicker(30 * time.Second)
	defer Tr.Stop()

	checklistUpdate := make(chan struct{})
	a.checklist.addListener(checklistUpdate)

	for {
		select {
		// Context terminated. Teardown now.
		case <-loopContext.Done():
			// Close base. Causes any blocked ReadFrom() calls to terminate.
			if err := base.Close(); err != nil {
				log.Error("failed to close base: %v", err)
			}
			return

		// New candidate added to checklist
		case <-checklistUpdate:
			log.Debug("Checklist state: %d", a.checklist.state)
			switch a.checklist.state {
			case checklistCompleted:
				if a.dataConn == nil {
					// Candidate base must match this loop's base
					if a.checklist.selected.local.base == base {
						// Use selected candidate.
						a.readyOnce.Do(func() {
							Ta.Stop()
							log.Info("Selected candidate pair: %s", a.checklist.selected)
							selected := a.checklist.selected
							a.dataConn = NewChannelConn(
								selected.local.base,
								dataIn,
								selected.remote.address.netAddr(),
							)
							a.ready <- a.dataConn
						})
					}
				}
			case checklistFailed:
				log.Fatal("Failed to connect to remote peer")
			}

		// [RFC8445 §6.1.4.2] Periodic connectivity check
		case <-Ta.C:
			if p := a.checklist.nextPair(); p != nil {
				log.Debug("Next candidate pair to check: %s\n", p)
				if err := a.checklist.sendCheck(
					p,
					a.username,
					a.remotePassword,
				); err != nil {
					log.Warn("Failed to send connectivity check: %s", err)
				}
			}

		// Keep-alive
		case <-Tr.C:
			// [RFC8445 §11] Send STUN binding indication.
			if p := a.checklist.selected; p != nil {
				p.local.base.sendStun(
					newStunBindingIndication(),
					p.remote.address.netAddr(),
					nil,
				)
			}
		}
	}

	// Should never get here
	log.Fatal("loop exited")
}

func (a *Agent) handleStun(msg *stunMessage, raddr net.Addr, base *Base) {
	if msg.method != stunBindingMethod {
		log.Fatalf("Unexpected STUN message: %s", msg)
	}

	switch msg.class {
	case stunRequest:
		a.handleStunRequest(msg, raddr, base)
	case stunIndication:
		// No-op
	case stunSuccessResponse, stunErrorResponse:
		log.Debug("Received unexpected STUN response: %s\n", msg)
	}
}

// [RFC8445 §7.3] Respond to STUN binding request by sending a success response.
func (a *Agent) handleStunRequest(req *stunMessage, raddr net.Addr, base *Base) {
	p := a.checklist.findPair(base, raddr)
	if p == nil {
		p = a.checklist.adoptPeerReflexiveCandidate(a.mid, base, raddr, req.getPriority())
	}
	if req.hasUseCandidate() && !p.nominated {
		log.Debug("Nominating %s\n", p.id)
		a.checklist.nominate(p)
	}

	resp := newStunBindingResponse(req.transactionID, raddr, a.localPassword)
	log.Debug("Sending response %s -> %s: %s\n", base.LocalAddr(), raddr, resp)
	if err := base.sendStun(resp, raddr, nil); err != nil {
		log.Warn("Failed to send STUN response: %s", err)
	}

	a.checklist.triggerCheck(p)
}
