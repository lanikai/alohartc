package ice

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

// https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20

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
	localAddr := a.conn.LocalAddr().(*net.UDPAddr)
	log.Println("Listening on UDP", localAddr)

//	a.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

//	// Default candidate for peers on the same LAN
//	lc := Candidate{typ: "host", component: 1}
//	lc.setAddress(localAddr)
//	a.computeFoundation(&lc)
//	a.computePriority(&lc)
//	a.localCandidates = append(a.localCandidates, lc)

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

func sameAddr(a, b net.Addr) bool {
	return a.Network() == b.Network() && a.String() == b.String()
}

func (a *Agent) EstablishConnection() (net.Conn, error) {
	//	a.conn.SetReadDeadline(time.Now().Add(time.Second))

	// Create candidate pairs.
	for _, local := range a.localCandidates {
		laddr := local.getAddress()
		for _, remote := range a.remoteCandidates {
			raddr := remote.getAddress()
			if laddr.Network() == raddr.Network() {
				cp := newCandidatePair(a.conn, local, remote, laddr, raddr)
				a.pairs = append(a.pairs, cp)
			}
		}
	}

	for i, cp := range a.pairs {
		log.Printf("Pair #%d: %s\n", i, cp)
	}
	return nil, nil
}
