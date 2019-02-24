package ice

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
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

	// Connection for the data stream.
	dataConn  *ChannelConn
	ready     chan *ChannelConn
	readyOnce sync.Once

	ctx context.Context
}

// Create a new ICE agent with the given username and passwords.
func NewAgent(ctx context.Context) *Agent {
	return &Agent{
		ready: make(chan *ChannelConn, 1),
		ctx:   ctx,
	}
}

func (a *Agent) Configure(mid, username, localPassword, remotePassword string) {
	a.mid = mid
	a.username = username
	a.localPassword = localPassword
	a.remotePassword = remotePassword
}

// On success, returns a net.Conn object from which data can be read/written.
func (a *Agent) EstablishConnection(lcand chan<- Candidate) (net.Conn, error) {
	if a.username == "" {
		return nil, errors.New("ICE agent not configured")
	}

	// TODO: Support multiple components
	//components := []int{1}
	component := 1

	bases, err := establishBases(component)
	if err != nil {
		return nil, err
	}

	// Start gathering condidates, trickling them to the remote agent via 'lcand'.
	go func() {
		err := a.gatherLocalCandidates(bases, lcand)
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
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("Failed to establish connection after 30 seconds")
	}
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
func (a *Agent) gatherLocalCandidates(bases []*Base, lcand chan<- Candidate) error {
	var wg sync.WaitGroup
	wg.Add(len(bases))
	for _, base := range bases {
		go func(base *Base) {
			log.Info("Gathering local candidates for base %s\n", base.address)
			// Host candidate for peers on the same LAN.
			hc := makeHostCandidate(a.mid, base)
			a.addLocalCandidate(hc)
			lcand <- hc

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
					lcand <- c
				}

				// TODO: TURN
			}

			wg.Done()
		}(base)
	}

	wg.Wait()
	close(lcand)
	return nil
}

func (a *Agent) loop(base *Base) {
	dataIn := make(chan []byte, 64)
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
			log.Debug("Checklist state: %d", a.checklist.state)
			switch a.checklist.state {
			case checklistCompleted:
				if a.dataConn == nil {
					// Use selected candidate.
					a.readyOnce.Do(func() {
						Ta.Stop()
						log.Info("Selected candidate pair: %s", a.checklist.selected)
						a.dataConn = createDataConn(a.ctx, a.checklist.selected, dataIn)
						a.ready <- a.dataConn
					})
				}
			case checklistFailed:
				log.Fatal("Failed to connect to remote peer")
			}

		case <-Ta.C: // Periodic check.
			p := a.checklist.nextPair()
			if p != nil {
				log.Debug("Next candidate to check: %s\n", p)
				err := a.checklist.sendCheck(p, a.username, a.remotePassword)
				if err != nil {
					log.Warn("Failed to send connectivity check: %s", err)
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
		log.Debug("Received unexpected STUN response: %s\n", msg)
	}
}

// [RFC8445 §7.3] Respond to STUN binding request by sending a success response.
func (a *Agent) handleStunRequest(req *stunMessage, raddr net.Addr, base *Base) {
	p := a.checklist.findPair(base, raddr)
	if p == nil {
		p = a.adoptPeerReflexiveCandidate(raddr, base, req.getPriority())
	}
	if req.hasUseCandidate() && !p.nominated {
		log.Debug("Nominating %s\n", p.id)
		a.checklist.nominate(p)
	}

	resp := newStunBindingResponse(req.transactionID, raddr, a.localPassword)
	log.Debug("Response %s -> %s: %s\n", base.LocalAddr(), raddr, resp)
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
		log.Fatalf("Expected candidate pair not present after creating peer reflexive candidate")
	}
	return p
}

func createDataConn(ctx context.Context, p *CandidatePair, dataIn chan []byte) *ChannelConn {
	base := p.local.base
	remoteAddr := p.remote.address.netAddr()
	dataOut := make(chan []byte, 32)
	dataConn := newChannelConn(dataIn, dataOut, base.LocalAddr(), remoteAddr)

	go func() {
		// Read constantly from the 'dataOut' channel, and forward to the underlying connection.
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-dataOut:
				if data == nil {
					log.Debug("%s: Channel closed\n", p.id)
					return
				}
				if _, err := base.WriteTo(data, remoteAddr); err != nil {
					log.Warn("%v", err)
					dataConn.Close()
				}
			}
		}
	}()

	return dataConn
}
