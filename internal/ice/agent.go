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

	ctx context.Context
}

// Create a new ICE agent with the given username and passwords.
func NewAgent(ctx context.Context) *Agent {
	return &Agent{
		lcand: make(chan Candidate, localCandidateBufferLength),
		ready: make(chan *ChannelConn, readySignalBufferLength),
		ctx:   ctx,
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
	for _, base := range bases {
		go a.loop(base)
	}

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

func (a *Agent) AddRemoteCandidate(desc, mid string) error {
	if desc == "" {
		// TODO: This should signal end of trickling.
		return nil
	}

	c := Candidate{mid: mid}
	if err := parseCandidateSDP(desc, &c); err != nil {
		return err
	}

	a.remoteCandidates = append(a.remoteCandidates, c)
	// Pair new remote candidate with all existing local candidates.
	a.checklist.addCandidatePairs(a.localCandidates, []Candidate{c})
	return nil
}

func (a *Agent) addLocalCandidate(c Candidate) {
	a.localCandidates = append(a.localCandidates, c)
	// Pair new local candidate with all existing remote candidates.
	a.checklist.addCandidatePairs([]Candidate{c}, a.remoteCandidates)
}

// Gather local candidates. Pass candidates to lcand as they become known.
func (a *Agent) gatherLocalCandidates(bases []*Base) error {
	var wg sync.WaitGroup
	wg.Add(len(bases))
	for _, base := range bases {
		go func(base *Base) {
			log.Infof("Gathering local candidates for base %s\n", base.address)
			// Host candidate for peers on the same LAN.
			hc := makeHostCandidate(a.mid, base)
			a.addLocalCandidate(hc)
			a.lcand <- hc

			if base.address.protocol == UDP && !base.address.linkLocal {
				// Query STUN server to get a server reflexive candidate.
				mappedAddress, err := base.queryStunServer(flagStunServer)
				if err != nil {
					log.Warnf("Failed to create STUN server candidate for base %s: %s\n", base.address, err)
				} else if mappedAddress == base.address {
					log.Warnf("Server-reflexive address for %s is same as base\n", base.address)
				} else {
					c := makeServerReflexiveCandidate(a.mid, mappedAddress, base, flagStunServer)
					a.addLocalCandidate(c)
					a.lcand <- c
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

func (a *Agent) loop(base *Base) {
	dataIn := make(chan []byte, packetBufferLength)
	go base.demuxStun(a.handleStun, dataIn)

	Ta := time.NewTicker(50 * time.Millisecond)
	defer Ta.Stop()

	Tr := time.NewTicker(30 * time.Second)
	defer Tr.Stop()

	checklistUpdate := make(chan struct{})
	a.checklist.addListener(checklistUpdate)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-checklistUpdate:
			log.Debugf("Checklist state: %d", a.checklist.state)
			switch a.checklist.state {
			case checklistCompleted:
				if a.dataConn == nil {
					// Use selected candidate.
					if a.checklist.selected.local.base == base {
						a.readyOnce.Do(func() {
							Ta.Stop()
							log.Infof("Selected candidate pair: %s", a.checklist.selected)
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

		case <-Ta.C: // Periodic check.
			p := a.checklist.nextPair()
			if p != nil {
				log.Debugf("Next candidate to check: %s\n", p)
				err := a.checklist.sendCheck(p, a.username, a.remotePassword)
				if err != nil {
					log.Warnf("Failed to send connectivity check: %s", err)
				}
			}

		// TODO: Triggered checks

		case <-Tr.C: // Keepalive.
			// [RFC8445 §11] Send STUN binding indication.
			p := a.checklist.selected
			if p != nil {
				p.local.base.sendStun(newStunBindingIndication(), p.remote.address.netAddr(), nil)
			}
		}
	}
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
		log.Debugf("Received unexpected STUN response: %s\n", msg)
	}
}

// [RFC8445 §7.3] Respond to STUN binding request by sending a success response.
func (a *Agent) handleStunRequest(req *stunMessage, raddr net.Addr, base *Base) {
	p := a.checklist.findPair(base, raddr)
	if p == nil {
		p = a.adoptPeerReflexiveCandidate(raddr, base, req.getPriority())
	}
	if req.hasUseCandidate() && !p.nominated {
		log.Debugf("Nominating %s\n", p.id)
		a.checklist.nominate(p)
	}

	resp := newStunBindingResponse(req.transactionID, raddr, a.localPassword)
	log.Debugf("Response %s -> %s: %s\n", base.LocalAddr(), raddr, resp)
	if err := base.sendStun(resp, raddr, nil); err != nil {
		log.Fatalf("Failed to send STUN response: %s", err)
	}

	// TODO: Enqueue triggered check
}

// [RFC8445 §7.3.1.3-4]
func (a *Agent) adoptPeerReflexiveCandidate(raddr net.Addr, base *Base, priority uint32) *CandidatePair {
	c := makePeerReflexiveCandidate(a.mid, raddr, base, priority)
	a.remoteCandidates = append(a.remoteCandidates, c)

	// Pair peer reflexive candidate with host candidate.
	hc := makeHostCandidate(a.mid, base)
	a.checklist.addCandidatePairs([]Candidate{hc}, []Candidate{c})

	p := a.checklist.findPair(base, raddr)
	if p == nil {
		log.Fatal("Expected candidate pair not present after creating peer reflexive candidate")
	}
	return p
}
