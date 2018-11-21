package ice

import (
	"fmt"
	"log"
	"net"
	"time"
)

// RFC 8445: https://tools.ietf.org/html/rfc8445

// In the language of the above specification, this is a Full implementation of a Controlled ICE
// agent, supporting a single component of a single data stream.
type Agent struct {
	username       string
	localPassword  string
	remotePassword string

	localCandidates  []Candidate
	remoteCandidates []Candidate

	checklist Checklist

	// Connection for the data stream.
	dataConn *ChannelConn
	ready chan *ChannelConn
}

// Create a new ICE agent with the given username and passwords.
func NewAgent(username, localPassword, remotePassword string) *Agent {
	a := &Agent{}
	a.username = username
	a.localPassword = localPassword
	a.remotePassword = remotePassword
	a.ready = make(chan *ChannelConn, 1)
	return a
}

// On success, returns a net.Conn object from which data can be read/written.
func (a *Agent) EstablishConnection(lcand chan<- string) (net.Conn, error) {
	// TODO: Handle multiple components
	component := 1
	base, err := createBase(component)
	if err != nil {
		return nil, err
	}
	log.Printf("Listening on %s\n", base.address)

	// Start gathering condidates, trickling them to the remote agent via 'lcand'.
	go func() {
		err := a.gatherLocalCandidates(base, lcand)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Begin connectivity checks.
	go a.loop(base)

	// Wait for a candidate to be selected.
	select {
	case conn := <-a.ready:
		return conn, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("Failed to establish connection after 30 seconds")
	}
}

func (a *Agent) AddRemoteCandidate(desc string) error {
	if desc == "" {
		return nil
	}

	c, err := parseCandidateSDP(desc)
	if err != nil {
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

// Gather local candidates. Pass candidate strings to lcand as they become known.
func (a *Agent) gatherLocalCandidates(base Base, lcand chan<- string) error {
	// Host candidate for peers on the same LAN.
	hc := makeHostCandidate(base)
	a.addLocalCandidate(hc)
	lcand <- hc.String()

	// Query STUN server to get a server reflexive candidate.
	stunServer := "stun2.l.google.com:19302"
	mappedAddress, err := a.queryStunServer(base, stunServer)
	if err != nil {
		log.Printf("Failed to create STUN server candidate for base %s\n", base.address)
	} else {
		c := makeServerReflexiveCandidate(mappedAddress, base, stunServer)
		a.addLocalCandidate(c)
		lcand <- c.String()
	}

	// TODO: IPv6, TURN, TCP, multiple local interfaces

	close(lcand)
	return nil
}

// Return the mapped address of the given base.
func (a *Agent) queryStunServer(base Base, stunServer string) (mappedAddr net.Addr, err error) {
	stunServerAddr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return
	}

	req := newStunBindingRequest("")
	trace("Sending to %s: %s\n", stunServer, req)

	done := make(chan error, 1)
	err = base.sendStun(req, stunServerAddr, func(resp *stunMessage, raddr net.Addr, base Base) {
		if resp.class == stunSuccessResponse {
			mappedAddr = resp.getMappedAddress()
			done <- nil
		} else {
			done <- fmt.Errorf("STUN server query failed: %s", resp)
		}
	})
	if err != nil {
		return
	}

	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		err = fmt.Errorf("Timed out waiting for response from %s", stunServer)
	}
	return
}

func (a *Agent) loop(base Base) {
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
		case <-checklistUpdate:
			trace("Checklist state: %d", a.checklist.state)
			switch a.checklist.state {
			case checklistCompleted:
				// Use selected candidate.
				if a.dataConn == nil {
					Ta.Stop()
					a.dataConn = createDataConn(a.checklist.selected, dataIn)
					a.ready <- a.dataConn
				}
			case checklistFailed:
				log.Fatalf("Failed to connect to remote peer")
			}

		case <-Ta.C: // Periodic check.
			p := a.checklist.nextPair()
			if p != nil {
				trace("Next candidate to check: %s\n", p)
				err := a.checklist.sendCheck(p, a.username, a.remotePassword)
				if err != nil {
					log.Fatalf("Failed to send connectivity check: %s", err)
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

func (a *Agent) handleStun(msg *stunMessage, raddr net.Addr, base Base) {
	if msg.method != stunBindingMethod {
		log.Fatalf("Unexpected STUN message: %s", msg)
	}

	switch msg.class {
	case stunRequest:
		a.handleStunRequest(msg, raddr, base)
	case stunIndication:
		// No-op
	case stunSuccessResponse, stunErrorResponse:
		trace("Received unexpected STUN response: %s\n", msg)
	}
}

// [RFC8445 §7.3] Respond to STUN binding request by sending a success response.
func (a *Agent) handleStunRequest(req *stunMessage, raddr net.Addr, base Base) {
	p := a.checklist.findPair(base, raddr)
	if p == nil {
		p = a.adoptPeerReflexiveCandidate(raddr, base, req.getPriority())
	}
	if req.hasUseCandidate() && !p.nominated {
		trace("Nominating %s\n", p.id)
		a.checklist.nominate(p)
	}

	resp := newStunBindingResponse(req.transactionID, raddr, a.localPassword)
	trace("Response %s -> %s: %s\n", base.LocalAddr(), raddr, resp)
	if err := base.sendStun(resp, raddr, nil); err != nil {
		log.Fatalf("Failed to send STUN response: %s", err)
	}

	// TODO: Enqueue triggered check
}

// [RFC8445 §7.3.1.3-4]
func (a *Agent) adoptPeerReflexiveCandidate(raddr net.Addr, base Base, priority uint32) *CandidatePair {
	c := makePeerReflexiveCandidate(raddr, base, priority)
	a.remoteCandidates = append(a.remoteCandidates, c)

	// Pair peer reflexive candidate with host candidate.
	hc := makeHostCandidate(base)
	a.checklist.addCandidatePairs([]Candidate{hc}, []Candidate{c})

	p := a.checklist.findPair(base, raddr)
	if p == nil {
		log.Fatalf("Expected candidate pair not present after creating peer reflexive candidate")
	}
	return p
}

func createDataConn(p *CandidatePair, dataIn chan []byte) *ChannelConn {
	base := p.local.base
	remoteAddr := p.remote.address.netAddr()
	dataOut := make(chan []byte, 32)
	dataConn := newChannelConn(dataIn, dataOut, base.LocalAddr(), remoteAddr)

	go func() {
		// Read constantly from the 'dataOut' channel, and forward to the underlying connection.
		for {
			data := <-dataOut
			if data == nil {
				trace("%s: Channel closed\n", p.id)
				return
			}
			if _, err := base.WriteTo(data, remoteAddr); err != nil {
				log.Println(err)
				dataConn.Close()
			}
		}
	}()

	return dataConn
}
