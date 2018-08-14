package ice

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
)

//
// https://tools.ietf.org/html/draft-ietf-mmusic-ice-sip-sdp-16

type Candidate struct {
	foundation string
	component  int
	protocol   string
	priority   uint
	ip         string
	port       int
	typ        string
	raddr      string  // Optional
	rport      int     // Optional

	// Extension attributes
	attrs []Attribute
}

type Attribute struct {
	name  string
	value string
}

const (
	CandidateTypeHost            = "host"
	CandidateTypePeerReflexive   = "prflx"
	CandidateTypeServerReflexive = "srflx"
	CandidateTypeRelay           = "relay"
)

func (c *Candidate) setAddress(addr net.Addr) {
	switch addr := addr.(type) {
	case *net.TCPAddr:
		c.protocol = "tcp"
		c.ip = addr.IP.String()
		c.port = addr.Port
	case *net.UDPAddr:
		c.protocol = "udp"
		c.ip = addr.IP.String()
		c.port = addr.Port
	default:
		panic(fmt.Errorf("Unsupported net.Addr type: %v", addr))
	}
}

func (c *Candidate) addAttribute(name, value string) {
	c.attrs = append(c.attrs, Attribute{name, value})
}

func (c Candidate) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "candidate:%s %d %s %d %s %d typ %s",
		c.foundation, c.component, c.protocol, c.priority, c.ip, c.port, c.typ)
	if c.raddr != "" {
		fmt.Fprintf(&b, " raddr %s", c.raddr)
	}
	if c.rport != 0 {
		fmt.Fprintf(&b, " rport %d", c.rport)
	}
	for _, a := range c.attrs {
		fmt.Fprintf(&b, " %s %s", a.name, a.value)
	}
	return b.String()
}

// An ICE candidate line is a string of the form
//   candidate:{foundation} {component-id} {transport} {priority} {address} {port} typ {type} ...
// See [draft-ietf-mmusic-ice-sip-sdp-16] Section 5.1
func parseCandidate(desc string) (c Candidate, err error) {
	r := strings.NewReader(desc)
	_, err = fmt.Fscanf(r, "candidate:%s %d %s %d %s %d typ %s",
		&c.foundation, &c.component, &c.protocol, &c.priority, &c.ip, &c.port, &c.typ)
	if err != nil {
		return
	}

	if c.component < 1 || c.component > 256 {
		return c, fmt.Errorf("Component ID out of range: %d", c.component)
	}

	// The rest of the candidate line consists of "name value" pairs.
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)
	var name string
	for scanner.Scan() {
		if name == "" {
			name = scanner.Text()
			continue
		}
		value := scanner.Text()
		switch name {
		case "typ":
			c.typ = value
		case "raddr":
			c.raddr = value
		case "rport":
			c.port, err = strconv.Atoi(value)
		default:
			c.addAttribute(name, value)
		}
		name = ""
	}
	if name != "" {
		return c, fmt.Errorf("Unmatched attribute name: %s", name)
	}

	return
}
