package webrtc

import (
	"fmt"
	"strings"
)

// Implementation of the Internet Connectivity Exchange (ICE) protocol, following RFC 5245bis
// (https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-20).

type IceAgent struct {
	localCandidates []IceCandidate
	remoteCandidates []IceCandidate
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

func (agent *IceAgent) AddRemoteCandidate(desc string) error {
	candidate, err := parseCandidateDesc(desc)
	if err != nil {
		return err
	}

	agent.remoteCandidates = append(agent.remoteCandidates, candidate)

	return nil
}

func parseCandidateDesc(desc string) (IceCandidate, error) {
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
