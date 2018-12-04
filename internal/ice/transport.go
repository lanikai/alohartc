package ice

import (
	"fmt"
	"net"
	"strings"
)

type TransportAddress struct {
	protocol string // Either "tcp" or "udp"
	ip       string
	port     int
}

func makeTransportAddress(addr net.Addr) TransportAddress {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return TransportAddress{"tcp", a.IP.String(), a.Port}
	case *net.UDPAddr:
		return TransportAddress{"udp", a.IP.String(), a.Port}
	default:
		panic("Unsupported net.Addr type: " + a.String())
	}
}

func (ta *TransportAddress) netAddr() (addr net.Addr) {
	hostport := fmt.Sprintf("%s:%d", ta.ip, ta.port)
	switch ta.protocol {
	case "tcp":
		addr, _ = net.ResolveTCPAddr("tcp", hostport)
	case "udp":
		addr, _ = net.ResolveUDPAddr("udp", hostport)
	}
	return
}

func (ta *TransportAddress) normalize() {
	ta.protocol = strings.ToLower(ta.protocol)
}

func (ta TransportAddress) String() string {
	return fmt.Sprintf("%s/%s:%d", ta.protocol, ta.ip, ta.port)
}
