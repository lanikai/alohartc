package ice

import (
	"bufio"
	"encoding/base32"
	"fmt"
	"hash/fnv"
	"net"
	"strings"
)

// An ICE candidate (either local or remote).
// See [RFC8445 §5.3] for a definition of fields.
type Candidate struct {
	// The data stream that this candidate belongs to, identified by its SDP "mid" field.
	mid string

	address    TransportAddress
	typ        string
	priority   uint32
	foundation string
	component  int
	attrs      []Attribute // Extension attributes

	base *Base // nil for remote candidates
}

type Attribute struct {
	name  string
	value string
}

const (
	hostType  = "host"
	srflxType = "srflx"
	prflxType = "prflx"
	relayType = "relay"
)

func makeHostCandidate(pt *PriorityTable, base *Base) Candidate {
	return Candidate{
		mid:        base.sdpMid,
		address:    base.address,
		typ:        hostType,
		priority:   computePriority(pt, hostType, base),
		foundation: computeFoundation(hostType, base.address, ""),
		component:  base.component,
		base:       base,
	}
}

func makeServerReflexiveCandidate(
	pt *PriorityTable,
	base *Base,
	mapped TransportAddress,
	stunServer string,
) Candidate {
	c := Candidate{
		mid:        base.sdpMid,
		address:    mapped,
		typ:        srflxType,
		priority:   computePriority(pt, srflxType, base),
		foundation: computeFoundation(srflxType, base.address, stunServer),
		component:  base.component,
		base:       base,
	}
	// [RFC5245 §15.1] requires raddr/rport. This is enforced by some browsers (e.g. Firefox).
	c.addAttribute("raddr", "0.0.0.0")
	c.addAttribute("rport", "0")
	return c
}

func makePeerReflexiveCandidate(
	pt *PriorityTable,
	base *Base,
	addr net.Addr,
	priority uint32,
) Candidate {
	ta := makeTransportAddress(addr)
	c := Candidate{
		mid:        base.sdpMid,
		address:    ta,
		typ:        prflxType,
		priority:   priority,
		foundation: computeFoundation(prflxType, ta, ""),
		component:  base.component,
		base:       base,
	}
	// [RFC5245 §15.1] requires raddr/rport. This is enforced by some browsers (e.g. Firefox).
	c.addAttribute("raddr", "0.0.0.0")
	c.addAttribute("rport", "0")
	return c
}

// [RFC8445 §5.1.2] Prioritizing Candidates
func computePriority(pt *PriorityTable, typ string, base *Base) uint32 {
	var localPref, typePref int
	switch typ {
	case hostType:
		typePref = 126
	case prflxType:
		typePref = 110
	case srflxType:
		typePref = 100
	case relayType:
		typePref = 0
	default:
		panic("Illegal candidate type: " + typ)
	}

	// Intermingle IPv4 and IPv6 candidates (see RFC8421 §4) by assigning IPv6
	// odd local preferences, and IPv4 even local preferences, with slight
	// preference towards IPv6.
	switch base.address.family {
	case IPv4:
		localPref = pt.ipv4
		pt.ipv4--
	case IPv6:
		localPref = pt.ipv6
		pt.ipv6--
	default:
		panic("Illegal address family")
	}

	return uint32((typePref << 24) + (localPref << 8) + ((256 - base.component) & 0xFF))
}

// [RFC8445 §5.1.1.3] The foundation must be unique for each tuple of
//     (candidate type, base IP address, protocol, STUN/TURN server)
func computeFoundation(typ string, baseAddress TransportAddress, stunServer string) string {
	fingerprint := fmt.Sprintf("%s/%s/%s", typ, baseAddress.protocol, baseAddress.ip)
	if stunServer != "" {
		fingerprint += "/" + stunServer
	}
	hash := fnv.New64()
	hash.Write([]byte(fingerprint))
	return base32.StdEncoding.EncodeToString(hash.Sum(nil))[0:8]
}

func (c *Candidate) addAttribute(name, value string) {
	c.attrs = append(c.attrs, Attribute{name, value})
}

func (c *Candidate) isReflexive() bool {
	return c.typ == srflxType || c.typ == prflxType
}

// Computes the priority of this candidate as if it were peer-reflexive, for use in connectivity
// checks.
func (c *Candidate) peerPriority(pt *PriorityTable) uint32 {
	return computePriority(pt, prflxType, c.base)
}

func (c *Candidate) sdpString() string {
	var b strings.Builder
	fmt.Fprintf(&b, "candidate:%s %d %s %d %s %d typ %s",
		c.foundation, c.component, c.address.protocol, c.priority, c.address.ip, c.address.port, c.typ)
	for _, a := range c.attrs {
		fmt.Fprintf(&b, " %s %s", a.name, a.value)
	}
	return b.String()
}

func (c *Candidate) Mid() string {
	return c.mid
}

func (c Candidate) String() string {
	return c.sdpString()
}

// An ICE candidate line is a string of the form
//   candidate:{foundation} {component-id} {protocol} {priority} {address} {port} typ {type} ...
// See https://tools.ietf.org/html/draft-ietf-mmusic-ice-sip-sdp-24#section-4.1
func ParseCandidate(desc, sdpMid string) (c Candidate, err error) {
	r := strings.NewReader(desc)

	var protocol, ip, port string
	_, err = fmt.Fscanf(r, "candidate:%s %d %s %d %s %s typ %s",
		&c.foundation, &c.component, &protocol, &c.priority, &ip, &port, &c.typ)
	if err != nil {
		return
	}
	if c.component < 1 || c.component > 256 {
		err = fmt.Errorf("Component ID out of range: %d", c.component)
		return
	}

	ipPort := net.JoinHostPort(ip, port)
	network := strings.ToLower(protocol)
	netAddr, err := resolveAddr(network, ipPort)
	if err != nil {
		return
	}
	c.address = makeTransportAddress(netAddr)

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
		default:
			c.addAttribute(name, value)
		}
		name = ""
	}
	if name != "" {
		err = fmt.Errorf("Unmatched attribute name: %s", name)
		return
	}

	c.mid = sdpMid
	return
}

func resolveAddr(network, address string) (net.Addr, error) {
	switch strings.ToLower(network) {
	case "tcp":
		return net.ResolveTCPAddr(network, address)
	case "udp":
		return net.ResolveUDPAddr(network, address)
	default:
		return nil, fmt.Errorf("Invalid network type: %s", network)
	}
}
