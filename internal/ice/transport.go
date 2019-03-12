package ice

import (
	"fmt"
	"net"
)

type TransportProtocol string
type IPAddressFamily int
type IPAddress [16]byte

const (
	TCP TransportProtocol = "tcp"
	UDP TransportProtocol = "udp"

	IPv4 IPAddressFamily = 4
	IPv6                 = 6
)

type TransportAddress struct {
	protocol  TransportProtocol
	family    IPAddressFamily
	ip        IPAddress
	port      int
	linkLocal bool
}

func makeTransportAddress(addr net.Addr) (ta TransportAddress) {
	var ip net.IP
	switch a := addr.(type) {
	case *net.TCPAddr:
		ta.protocol = TCP
		ip = a.IP
		ta.port = a.Port
	case *net.UDPAddr:
		ta.protocol = UDP
		ip = a.IP
		ta.port = a.Port
	default:
		panic("Unsupported net.Addr: " + a.String())
	}

	if ip4 := ip.To4(); ip4 != nil {
		ta.family = IPv4
	} else {
		ta.family = IPv6
	}
	copy(ta.ip[:], ip.To16())
	ta.linkLocal = ip.IsLinkLocalUnicast()
	return
}

func (ta *TransportAddress) netAddr() net.Addr {
	switch ta.protocol {
	case TCP:
		return &net.TCPAddr{ta.ip.netIP(), ta.port, ""}
	case UDP:
		return &net.UDPAddr{ta.ip.netIP(), ta.port, ""}
	default:
		panic("Unsupported transport protocol: " + ta.protocol)
	}
}

func (ta TransportAddress) String() string {
	if ta.family == IPv6 {
		return fmt.Sprintf("%s/[%s]:%d", ta.protocol, ta.ip, ta.port)
	} else {
		return fmt.Sprintf("%s/%s:%d", ta.protocol, ta.ip, ta.port)
	}
}

func (ip IPAddress) netIP() net.IP {
	return net.IP(ip[:])
}

func (ip IPAddress) String() string {
	return ip.netIP().String()
}
