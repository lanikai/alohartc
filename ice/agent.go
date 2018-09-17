package ice

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"time"
)

// RFC8445: https://tools.ietf.org/html/rfc8445

// In the language of the above specification, this is a Full implementation of a Controlled ICE
// agent, supporting a single component of a data stream. It does not (yet) implement candidate
// trickling.
type Agent struct {
	username       string
	localPassword  string
	remotePassword string

	localCandidates  []Candidate
	remoteCandidates []Candidate

	pairs []*CandidatePair

	conn *net.UDPConn

	// Connection for the data stream.
	dataconn *ChannelConn

	foundationFingerprints []string
}

// Create a new ICE agent with the given username and passwords.
func NewAgent(username, localPassword, remotePassword string) *Agent {
	a := &Agent{}
	a.username = username
	a.localPassword = localPassword
	a.remotePassword = remotePassword
	return a
}

func (a *Agent) AddRemoteCandidate(desc string) error {
	candidate, err := parseCandidate(desc)
	if err != nil {
		return err
	}

	a.remoteCandidates = append(a.remoteCandidates, candidate)
	return nil
}

func (a *Agent) GatherLocalCandidates() ([]Candidate, error) {
	localIP, err := getLocalIP()
	if err != nil {
		log.Println("Failed to get local address")
		return nil, err
	}

	// Listen on an arbitrary UDP port.
	listenAddr := &net.UDPAddr{IP: localIP, Port: 0}
	a.conn, err = net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return nil, err
	}
	localAddr := a.conn.LocalAddr()
	log.Println("Listening on UDP", localAddr)

	// Default candidate for peers on the same LAN.
	lc := Candidate{typ: "host", component: 1}
	lc.setAddress(localAddr)
	a.computeFoundation(&lc)
	a.computePriority(&lc)
	a.localCandidates = append(a.localCandidates, lc)

	// Query STUN server to get a server reflexive candidate.
	stunServerAddr, err := net.ResolveUDPAddr("udp", "stun2.l.google.com:19302")
	if err != nil {
		return nil, err
	}
	sc, err := getStunCandidate(a.conn, stunServerAddr)
	if err != nil {
		return nil, err
	} else {
		a.computeFoundation(sc)
		a.computePriority(sc)
		a.localCandidates = append(a.localCandidates, *sc)
	}

	return a.localCandidates, nil
}

// Get the IP address of this machine.
func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}

// [RFC8445] Section 5.1.1.3. Computing Foundations
// The foundation must be unique for each tuple of
//   (candidate type, base IP address, protocol, STUN/TURN server)
// We compute a "fingerprint" that encodes this information, and assign each fingerprint
// an integer based on the order it was encountered.
func (a *Agent) computeFoundation(c *Candidate) {
	fingerprint := fmt.Sprintf("%s/%s/%s/nil", c.typ, c.ip, c.protocol)
	for n, f := range a.foundationFingerprints {
		if f == fingerprint {
			c.foundation = strconv.Itoa(n)
			return
		}
	}

	// Fingerprint is new. Save it and return the next available index.
	n := len(a.foundationFingerprints)
	a.foundationFingerprints = append(a.foundationFingerprints, fingerprint)
	c.foundation = strconv.Itoa(n)
}

// [RFC8445] Section 5.1.2. Prioritizing Candidates
func (a *Agent) computePriority(c *Candidate) {
	var typePref int
	switch c.typ {
	case "host":
		typePref = 126
	case "prflx", "srflx":
		typePref = 110
	case "relay":
		typePref = 0
	}

	// Assume there's a single local IP address
	localPref := 65535

	c.priority = uint((typePref << 24) + (localPref << 8) + (256 - c.component))
}

func sameAddr(a, b net.Addr) bool {
	return a.Network() == b.Network() && a.String() == b.String()
}

func (a *Agent) EstablishConnection() (conn net.Conn, err error) {
	// Create candidate pairs.
	for _, local := range a.localCandidates {
		for _, remote := range a.remoteCandidates {
			if remote.protocol == local.protocol {
				cp := newCandidatePair(len(a.pairs), local, remote)
				a.pairs = append(a.pairs, cp)
			}
		}
	}

	for _, cp := range a.pairs {
		log.Println(cp)
	}

	ready := make(chan *ChannelConn, 1)
	go a.loop(ready)

	select {
	case conn = <-ready:
	case <-time.After(15 * time.Second):
		err = fmt.Errorf("Failed to establish connection after 15 seconds")
	}

	return conn, err
}


func (a *Agent) loop(ready chan<- *ChannelConn) {
	datain := make(chan []byte, 32)
	dataout := make(chan []byte, 32)
	go func() {
		// Read constantly from the 'dataout' channel, and forward to the underlying connection.
		for {
			a.conn.Write(<-dataout)
		}
	}()

	buf := make([]byte, 1500)
	for {
		// Read continuously from UDP connection
		a.conn.SetReadDeadline(time.Now().Add(5*time.Second))
		n, raddr, err := a.conn.ReadFrom(buf)
		if err != nil {
			log.Fatal(err)
		}
		data := buf[0:n]

		// Decide which candidate pair the packet should be associated with.
		var cp *CandidatePair
		for i := range a.pairs {
			if sameAddr(raddr, a.pairs[i].remote.Addr()) {
				cp = a.pairs[i]
				break
			}
		}

		if cp == nil {
			// No matching candidate pair found. Create a new one.
			local := a.localCandidates[0]
			remote := makePeerCandidate(local.component, raddr)
			a.computeFoundation(&remote)
			a.computePriority(&remote)
			a.remoteCandidates = append(a.remoteCandidates, remote)

			cp = newCandidatePair(len(a.pairs), local, remote)
			log.Printf("Candidate pair #%d: %s", len(a.pairs), cp)
			a.pairs = append(a.pairs, cp)
		}

		msg, err := parseStunMessage(data)
		if err != nil {
			log.Fatal(err)
		}

		if msg != nil {
			a.handleStun(cp, msg)
			if a.dataconn == nil && cp.state == cpSucceeded {
				// We have selected a candidate pair.
				a.dataconn = newChannelConn(datain, dataout, cp.local.Addr(), cp.remote.Addr())
				ready <- a.dataconn
			}
		} else if a.dataconn != nil {
			datain <- data
		} else {
			log.Panicf("Received data packet before ICE candidate pair selected: %s", data)
		}
	}
}

func (a *Agent) handleStun(cp *CandidatePair, msg *stunMessage) {
	log.Printf("CP #%d: Received %s\n", cp.seq, msg)

	switch msg.class {
	case stunRequest:
		cp.state = cpInProgress

		// Send a response.
		resp := newStunBindingResponse(msg.transactionID, cp.remote.Addr(), a.localPassword)
		log.Printf("CP #%d: Sending %s\n", cp.seq, resp)
		a.conn.WriteTo(resp.Bytes(), cp.remote.Addr())

		// Followed by a binding request of our own.
		req := newStunBindingRequest(msg.transactionID)
		req.addAttribute(stunAttrUsername, []byte(a.username))
		req.addAttribute(stunAttrIceControlled, []byte{1, 2, 3, 4, 5, 6, 7, 8})
		req.addMessageIntegrity(a.remotePassword)
		req.addFingerprint()
		log.Printf("CP #%d: Sending %s\n", cp.seq, req)
		a.conn.WriteTo(req.Bytes(), cp.remote.Addr())
	case stunSuccessResponse:
		cp.state = cpSucceeded
		log.Printf("CP #%d: Succeeded\n", cp.seq)
	case stunErrorResponse:
		cp.state = cpFailed
		log.Printf("CP #%d: Failed\n", cp.seq)
	case stunIndication:
		// Ignore these.
	}
}
