package ice

import (
	"fmt"
	"net"
)

// Transport protocol is either "udp" or "tcp" (support for the latter is
// currently incomplete).
type TransportProtocol string

// Address family is either IPv4, IPv6, or Unresolved (indicating that the
// stored IPAddress hasn't actually been resolved yet).
type AddressFamily int

// The net package represents an IP address as a byte slice, either 4 bytes for
// an IPv4 address or 16 bytes for IPv6. But []byte cannot be compared with ==,
// and for simplicity we want TransportAddress instances to be comparable. So
// instead we represent an IP address a string (which is really just an
// immutable byte slice), with the understanding that it will have length 4 for
// IPv4, length 16 for IPv6, or end with ".local" for unresolved ephemeral mDNS
// hostnames.
type IPAddress string

const (
	TCP TransportProtocol = "tcp"
	UDP TransportProtocol = "udp"

	Unresolved AddressFamily = 0
	IPv4       AddressFamily = 4
	IPv6       AddressFamily = 6
)

type TransportAddress struct {
	protocol  TransportProtocol
	family    AddressFamily
	ip        IPAddress
	port      int
	linkLocal bool
}

// Construct a TransportAddress from a net.Addr instance.
func makeTransportAddress(addr net.Addr) (ta TransportAddress) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		ta.protocol = TCP
		ta.setIP(a.IP)
		ta.port = a.Port
	case *net.UDPAddr:
		ta.protocol = UDP
		ta.setIP(a.IP)
		ta.port = a.Port
	default:
		panic("Unsupported net.Addr: " + a.String())
	}

	return
}

func (ta *TransportAddress) resolved() bool {
	return ta.family != Unresolved
}

func (ta *TransportAddress) setIP(ip net.IP) {
	if ip == nil {
		ta.family = Unresolved
		ta.ip = IPAddress("")
	} else if ip4 := ip.To4(); ip4 != nil {
		ta.family = IPv4
		ta.ip = IPAddress(ip4)
	} else {
		ta.family = IPv6
		ta.ip = IPAddress(ip)
	}
	ta.linkLocal = ip.IsLinkLocalUnicast()
}

func (ta *TransportAddress) netAddr() net.Addr {
	var ip net.IP
	if ta.resolved() {
		ip = net.IP(ta.ip)
	}
	switch ta.protocol {
	case TCP:
		return &net.TCPAddr{ip, ta.port, ""}
	case UDP:
		return &net.UDPAddr{ip, ta.port, ""}
	default:
		panic("Unsupported transport protocol: " + ta.protocol)
	}
}

func (ta *TransportAddress) displayIP() string {
	if ta.resolved() {
		return net.IP(ta.ip).String()
	}
	return string(ta.ip)
}

func (ta TransportAddress) String() string {
	if ta.family == IPv6 {
		return fmt.Sprintf("%s/[%s]:%d", ta.protocol, ta.displayIP(), ta.port)
	} else {
		return fmt.Sprintf("%s/%s:%d", ta.protocol, ta.displayIP(), ta.port)
	}
}
