package webrtc

import (
	"fmt"
	"log"
	"net"
	"strings"
)

// Implementation of the Internet Connectivity Exchange (ICE) protocol, following RFC 5245bis
// (https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20).

type IceAgent struct {
	localCandidates []IceCandidate
	remoteCandidates []IceCandidate

	localAddr *net.UDPAddr
	conn net.PacketConn
}

type IceCandidate struct {
	foundation string
	component int
	protocol string
	priority uint
	ip string
	port int
	attrs map[string]string
	attrkeys []string  // for iterating in insertion order
}

func NewIceAgent() *IceAgent {
	return &IceAgent{}
}

func (agent *IceAgent) AddRemoteCandidate(desc string) error {
	candidate, err := parseCandidate(desc)
	if err != nil {
		return err
	}

	agent.remoteCandidates = append(agent.remoteCandidates, candidate)
	return nil
}

func (agent *IceAgent) GatherCandidates() ([]IceCandidate, error) {
	// Listen on an arbitrary UDP port.
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	log.Println("Listening on UDP", localAddr)

	c := IceCandidate{
		foundation: "0",
		component: 1,
		protocol: "udp",
		priority: 100,
		ip: localAddr.IP.String(),
		port: localAddr.Port,
	}
	c.setAttr("typ", "host")

	// TODO: Query public STUN server to get server reflexive candidates.
	c2 := IceCandidate{
		foundation: "0",
		component: 1,
		protocol: "udp",
		priority: 99,
		ip: "127.0.0.1",
		port: localAddr.Port,
	}
	c2.setAttr("typ", "host")

	agent.localAddr = localAddr
	agent.conn = conn
	agent.localCandidates = append(agent.localCandidates, c, c2)

	return agent.localCandidates, nil
}

func parseCandidate(desc string) (IceCandidate, error) {
	c := IceCandidate{}
	n, err := fmt.Sscanf(desc, "candidate:%s %d %s %d %s %d",
		&c.foundation, &c.component, &c.protocol, &c.priority, &c.ip, &c.port)
	if err != nil { return c, err }

	kv := strings.Fields(desc)[n:]
	if len(kv) % 2 != 0 {
		return c, fmt.Errorf("Invalid candidate description: %s", desc)
	}

	for i := 0; i < len(kv); i += 2 {
		key, val := kv[i], kv[i+1]
		c.setAttr(key, val)
	}

	return c, nil
}

func (c *IceCandidate) setAttr(key string, val string) {
	if c.attrs == nil {
		c.attrs = make(map[string]string)
	}
	c.attrs[key] = val
	c.attrkeys = append(c.attrkeys, key)
}

func (c IceCandidate) String() string {
	var b strings.Builder
	b.WriteString(
		fmt.Sprintf("candidate:%s %d %s %d %s %d",
		c.foundation, c.component, c.protocol, c.priority, c.ip, c.port))
	for _, key := range c.attrkeys {
		val := c.attrs[key]
		b.WriteString(" " + key + " " + val)
	}
	return b.String()
}
