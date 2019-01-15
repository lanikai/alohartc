package ice

import (
	"fmt"
	"net"
	"strings"
)

type TransportProtocol string
type IPAddressFamily int

const (
	TCP TransportProtocol = "tcp"
	UDP                   = "udp"

	IPv4 IPAddressFamily = 4
	IPv6                 = 6
)

type TransportAddress struct {
	protocol TransportProtocol
	ip       string
	port     int
	family   IPAddressFamily
}

// @param network  Network string as in the "net" package, e.g. "tcp" or "udp4"
// @param ip       IPv4 or IPv6 address in standard printed format
// @param port     Port
func newTransportAddress(network string, ip string, port int) TransportAddress {
	return TransportAddress{
		protocol: getTransportProtocol(network),
		ip: ip,
		port: port,
		family: getIPAddressFamily(ip),
	}
}

func makeTransportAddress(addr net.Addr) TransportAddress {
	switch a := addr.(type) {
	case *net.TCPAddr:
		ip := a.IP.String()
		return TransportAddress{TCP, ip, a.Port, getIPAddressFamily(ip)}
	case *net.UDPAddr:
		ip := a.IP.String()
		return TransportAddress{UDP, ip, a.Port, getIPAddressFamily(ip)}
	default:
		panic("Unsupported net.Addr type: " + a.String())
	}
}

func (ta *TransportAddress) netAddr() net.Addr {
	network := fmt.Sprintf("%s%d", ta.protocol, ta.family)
	hostport := fmt.Sprintf("%s:%d", ta.ip, ta.port)
	switch ta.protocol {
	case "tcp":
		addr, _ := net.ResolveTCPAddr(network, hostport)
		return addr
	case "udp":
		addr, _ := net.ResolveUDPAddr(network, hostport)
		return addr
	default:
		panic("Unsupported transport protocol: " + ta.protocol)
	}
}

func getTransportProtocol(network string) TransportProtocol {
	switch strings.ToLower(network) {
	case "tcp", "tcp4", "tcp6":
		return TCP
	case "udp", "udp4", "udp6":
		return UDP
	default:
		panic("Unsupported network: " + network)
	}
}

func getIPAddressFamily(ip string) IPAddressFamily {
	if strings.ContainsRune(ip, ':') {
		return IPv6
	} else {
		return IPv4
	}
}

func (ta TransportAddress) String() string {
	if ta.family == IPv6 {
		return fmt.Sprintf("%s/[%s]:%d", ta.protocol, ta.ip, ta.port)
	} else {
		return fmt.Sprintf("%s/%s:%d", ta.protocol, ta.ip, ta.port)
	}
}
