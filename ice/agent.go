package ice

import (
	"fmt"
	"log"
	"net"
	"strconv"
)


type Agent struct {
	localCandidates []Candidate
	remoteCandidates []Candidate

	conn *net.UDPConn
	localAddr *net.UDPAddr

	foundationFingerprints []string
}

// Create a new ICE agent.
func NewAgent() *Agent {
	return &Agent{}
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
	a.localAddr = a.conn.LocalAddr().(*net.UDPAddr)
	log.Println("Listening on UDP", a.localAddr)

//	a.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// Local host candidate
	lc := Candidate{typ: "host", component: 1}
	lc.setAddress(a.localAddr)
	a.computeFoundation(&lc)
	a.computePriority(&lc)
	a.localCandidates = append(a.localCandidates, lc)

	stunServerAddr, err := net.ResolveUDPAddr("udp", "stun2.l.google.com:19302")
	if err != nil {
		return nil, err
	}

	sc, err := getStunCandidate(a.conn, stunServerAddr)
	if err != nil {
		log.Println("Failed to query STUN server:", err)
	} else {
		a.computeFoundation(sc)
		a.computePriority(sc)
		a.localCandidates = append(a.localCandidates, *sc)
	}

	return a.localCandidates, nil
}

func getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}

// [draft-ietf-ice-rfc5245bis-20] Section 5.1.1.3. Computing Foundations
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

// [draft-ietf-ice-rfc5245bis-20] Section 5.1.2. Prioritizing Candidates
func (a *Agent) computePriority(c *Candidate) {
	var typePref int
	switch c.typ {
	case "host":
		typePref = 126
	case "prflx":
		typePref = 110
	case "srflx":
		typePref = 110
	case "relay":
		typePref = 0
	}

	// Assume there's a single local IP address
	localPref := 65535

	c.priority = uint((typePref << 24) + (localPref << 8) + (256 - c.component))
}

func (a *Agent) EstablishConnection(username, password string) (net.Conn, error) {
//	a.conn.SetReadDeadline(time.Now().Add(time.Second))

	buf := make([]byte, 1500)

	// Act as STUN server, awaiting binding request from remote ICE agent.
	n, raddr, err := a.conn.ReadFrom(buf)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	req, err := parseStunMessage(buf[0:n])
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Incoming ICE request:", req.String())

	// Send response.
	resp := newStunBindingResponse(req.transactionID)
	resp.setXorMappedAddress(raddr.(*net.UDPAddr))
	resp.addMessageIntegrity(password)
	resp.addFingerprint()
	log.Println("Outgoing ICE response:", resp.String())
	a.conn.WriteTo(resp.Bytes(), raddr)


/*
	// Now act as STUN client.
	req2 := newStunBindingRequest()
	req2.addAttribute(stunAttrUsername, []byte(username))
	req2.addAttribute(stunAttrIceControlled, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	req2.addMessageIntegrity(password)
	req2.addFingerprint()
	log.Println("Outgoing ICE request:", req2.String())
	a.conn.WriteTo(req2.Bytes(), raddr)

	n, _, err = a.conn.ReadFrom(buf)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	resp2, err := parseStunMessage(buf[0:n])
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Incoming ICE response:", resp2.String())
*/

	return a.conn, nil
}
