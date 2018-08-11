package webrtc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
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

func (agent *IceAgent) GatherCandidates() ([]IceCandidate, error) {
	localAddr, err := getLocalAddr()
	if err != nil {
		log.Println("Failed to get local address")
		return nil, err
	}

	// Listen on an arbitrary UDP port.
	conn, err := net.ListenPacket("udp4", localAddr.IP.String() + ":0")
	if err != nil {
		return nil, err
	}
	localAddr = conn.LocalAddr().(*net.UDPAddr)
	log.Println("Listening on UDP", localAddr)

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	agent.conn = conn
	agent.localAddr = localAddr

	// Local host candidate
	lc := IceCandidate{
		foundation: "0",
		component: 1,
		protocol: "udp",
		priority: 126,
		ip: localAddr.IP.String(),
		port: localAddr.Port,
	}
	lc.setAttr("typ", "host")
	agent.localCandidates = append(agent.localCandidates, lc)

	stunServerAddr, err := net.ResolveUDPAddr("udp", "stun2.l.google.com:19302")
	if err != nil {
		return nil, err
	}

	rc, err := stunBindingExchange(conn, stunServerAddr)
	if err != nil {
		log.Println("Failed to query STUN server:", err)
	} else {
		agent.localCandidates = append(agent.localCandidates, *rc)
	}

	return agent.localCandidates, nil
}

func getLocalAddr() (*net.UDPAddr, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr), nil
}

// Send a STUN binding request to the given address, and await a binding response.
func stunBindingExchange(conn net.PacketConn, addr net.Addr) (*IceCandidate, error) {
	req := newStunBindingRequest()
	log.Printf("STUN binding request: %x", req.Bytes())
	_, err := conn.WriteTo(req.Bytes(), addr)
	if err != nil {
		log.Println("Failed to send STUN binding request:", err)
		return nil, err
	}
	log.Println("Sent STUN binding request")

	buf := make([]byte, 1500)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		log.Println("Did not receive STUN binding response:", err)
		return nil, err
	}
	log.Println("Received STUN binding response")

	resp, err := parseStunMessage(buf[:n])
	if err != nil {
		return nil, err
	}
	log.Println(resp)

	if ! bytes.Equal(resp.transactionID, req.transactionID) {
		return nil, fmt.Errorf("Unknown transaction ID in STUN binding response: %s", resp.transactionID)
	}
	if resp.class != stunSuccessResponse {
		return nil, fmt.Errorf("STUN binding response is not successful: %d", resp.class)
	}

	// Find XOR-MAPPED-ADDRESS attribute in the response.
	for _, attr := range resp.attributes {
		log.Printf("STUN attribute: %#x %d %s", attr.Type, attr.Length, hex.EncodeToString(attr.Value))
	}
	mappedAddr, err := resp.getMappedAddress()
	if err != nil {
		return nil, err
	}
	log.Printf("mappedAddr = %s", mappedAddr)

	c := &IceCandidate{
		foundation: "1",
		component: 1,
		protocol: "udp",
		priority: 110,
		ip: mappedAddr.IP.String(),
		port: mappedAddr.Port,
	}
	c.setAttr("typ", "host")
	return c, nil
}

func (agent *IceAgent) CheckConnectivity() error {
	buf := make([]byte, 1500)
	for {
		log.Println("CheckConnectivity:")
		n, raddr, err := agent.conn.ReadFrom(buf)
		if err != nil {
			return err
		}
		log.Println(buf[:n])

		msg, err := parseStunMessage(buf[:n])
		if err != nil {
			return err
		}
		if msg == nil {
			continue
		}

		log.Println("Received STUN message from", raddr)
		log.Println(msg)

		_, err = stunBindingExchange(agent.conn, raddr)
		return err
	}
}
