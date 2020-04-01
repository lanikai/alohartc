package mdns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// High bit of CLASS bit in questions and resource records, repurposed by mDNS.
	classMask = 1 << 15

	// Query interval when waiting for a name to resolve.
	initialQueryInterval = 100 * time.Millisecond

	// Initial size limit before pruning the cache.
	initialPruneSize = 8
)

// Multicast DNS addresses, per RFC 6762.
var mdnsGroupAddr4 = &net.UDPAddr{
	IP:   net.ParseIP("224.0.0.251"),
	Port: 5353,
}
var mdnsGroupAddr6 = &net.UDPAddr{
	IP:   net.ParseIP("ff02::fb"),
	Port: 5353,
}

type Client struct {
	// UDP connections bound to IPv4 and IPv6 mDNS multicast addresses.
	conn4 *net.UDPConn
	conn6 *net.UDPConn

	// Indicates a clean shutdown.
	stopped bool

	// Cache of ephemeral .local hostname resolutions. The key is the UUID part
	// of the domain (i.e. without the ".local." suffix).
	cache map[string]*record

	// When the cache reaches this size, go through and prune expired records.
	pruneSize int

	sync.Mutex
}

func NewClient() (*Client, error) {
	// Listen on both IPv4 and IPv6 mDNS multicast addresses.
	conn4, err := net.ListenMulticastUDP("udp4", nil, mdnsGroupAddr4)
	if err != nil {
		return nil, err
	}
	conn6, err := net.ListenMulticastUDP("udp6", nil, mdnsGroupAddr6)
	if err != nil {
		conn4.Close()
		return nil, err
	}

	c := &Client{
		conn4:     conn4,
		conn6:     conn6,
		stopped:   false,
		cache:     make(map[string]*record),
		pruneSize: initialPruneSize,
	}

	// Enable multicast loopback, for the case when we're running on the same
	// host as the remote peer (mostly useful for testing).
	pconn4 := ipv4.NewPacketConn(conn4)
	if err := pconn4.SetMulticastLoopback(true); err != nil {
		c.Close()
		return nil, err
	}
	pconn6 := ipv6.NewPacketConn(conn6)
	if err := pconn6.SetMulticastLoopback(true); err != nil {
		c.Close()
		return nil, err
	}

	// Start read loops to handle incoming mDNS messages.
	go c.readLoop(conn4)
	go c.readLoop(conn6)

	return c, nil
}

func (c *Client) Close() error {
	c.Lock()
	defer c.Unlock()

	c.stopped = true
	if c.conn4 != nil {
		c.conn4.Close()
	}
	if c.conn6 != nil {
		c.conn6.Close()
	}
	return nil
}

func (c *Client) readLoop(conn *net.UDPConn) {
	log.Trace(3, "read loop start (%s)", conn.LocalAddr())
	defer log.Trace(3, "read loop end (%s)", conn.LocalAddr())

	buf := make([]byte, 1500)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if n > 0 {
			c.handleMessage(buf[:n], src, conn)
		}

		if err != nil {
			// If Close() has been called, this error is expected and normal.
			if !c.stopped {
				log.Error("read error (%s): %v", conn.LocalAddr(), err)
			}
			return
		}
	}
}

func (c *Client) handleMessage(msg []byte, src *net.UDPAddr, conn *net.UDPConn) {
	var p dnsmessage.Parser
	hdr, err := p.Start(msg)
	if err != nil {
		log.Warn("invalid DNS message: %v", err)
		return
	}

	log.Trace(3, "message header = %+v", hdr)
	if hdr.OpCode != 0 {
		// Ignore non-zero OPCODE: https://tools.ietf.org/html/rfc6762#section-18.3
		return
	}

	// Parse questions.
	for {
		q, err := p.Question()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			log.Debug("invalid question: %v", err)
			break
		}
		log.Debug("received question: %v", q)

		c.handleQuestion(&q, src, conn)
	}

	// Parse answers.
	for {
		a, err := p.Answer()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			log.Debug("invalid answer: %v", err)
			break
		}
		log.Debug("received answer: %v", a)

		c.handleAnswer(&a)
	}
}

// Inspect an mDNS query and respond if appropriate.
func (c *Client) handleQuestion(q *dnsmessage.Question, src *net.UDPAddr, conn *net.UDPConn) {
	// Extract domain name, excluding the final "." character.
	name := q.Name.String()[:q.Name.Length-1]
	if !isEphemeralLocalDomain(name) {
		return
	}

	// Strip ".local" suffix to get the UUID part of the ephemeral domain.
	uuid := name[:len(name)-6]

	// Only respond if we have an authoritative record for this domain.
	c.Lock()
	r, found := c.cache[uuid]
	c.Unlock()
	if found && r.ours && q.Type == r.Type() && time.Now().Before(r.expires) {
		dst := src
		if (q.Class & classMask) == 0 {
			// High bit of QCLASS means a unicast response is requested.
			dst = conn.LocalAddr().(*net.UDPAddr)
		}
		log.Debug("responding to %v with %v", q, r.ip)
		if err := c.sendResponse(r, dst, conn); err != nil {
			log.Warn("failed to send response: %v", err)
		}
	}
}

func (c *Client) handleAnswer(a *dnsmessage.Resource) {
	if (a.Header.Class &^ classMask) != dnsmessage.ClassINET {
		log.Trace(3, "ignoring answer of class %d", a.Header.Class)
		return
	}

	// We're only interested in mDNS-ICE ephemeral domains.
	name := a.Header.Name.String()[:a.Header.Name.Length-1] // strip final "."
	if !isEphemeralLocalDomain(name) {
		log.Trace(3, "ignoring answer for %s", name)
		return
	}

	// Extract the IP address from the resource record.
	var ip net.IP
	switch res := a.Body.(type) {
	case *dnsmessage.AResource:
		ip = append(ip, res.A[:]...)
	case *dnsmessage.AAAAResource:
		ip = append(ip, res.AAAA[:]...)
	default:
		log.Trace(3, "ignoring answer of type %T", a.Body)
		return
	}

	uuid := name[:len(name)-6]
	expires := time.Now().Add(time.Duration(a.Header.TTL) * time.Second)

	c.Lock()
	if r, found := c.cache[uuid]; found {
		// Received an answer to an earlier query. Update the record and notify
		// any listeners.
		r.Update(ip, expires)
	} else {
		// Doesn't answer any of our current pending queries, but cache it
		// anyway in case we want it later.
		c.cache[uuid] = &record{
			name:    a.Header.Name,
			ip:      ip,
			expires: expires,
			ours:    false,
		}
	}
	c.Unlock()

	c.maybePruneCache()
}

func (c *Client) sendResponse(r *record, dst *net.UDPAddr, conn *net.UDPConn) error {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID:            0, // mDNS query ID is always 0
		Response:      true,
		Authoritative: true,
		RCode:         dnsmessage.RCodeSuccess,
	})
	b.EnableCompression()
	b.StartAnswers()
	resHdr := dnsmessage.ResourceHeader{
		Name:  r.name,
		Class: dnsmessage.ClassINET,
		TTL:   uint32(time.Until(r.expires) / time.Second),
	}
	if ip4 := r.ip.To4(); ip4 != nil {
		var res dnsmessage.AResource
		copy(res.A[:], ip4)
		b.AResource(resHdr, res)
	} else {
		var res dnsmessage.AAAAResource
		copy(res.AAAA[:], r.ip)
		b.AAAAResource(resHdr, res)
	}

	msg, err := b.Finish()
	if err != nil {
		return err
	}

	log.Debug("sending response to %s", r.ip)
	if _, err := conn.WriteTo(msg, dst); err != nil {
		return err
	}

	return nil
}

func (c *Client) sendQuery(r *record) error {
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID: 0, // mDNS query ID is always 0
	})
	b.EnableCompression()
	b.StartQuestions()
	// An ephemeral domain can be for an IPv4 or IPv6 address, and we don't have
	// any way of knowing which. So query for both.
	b.Question(dnsmessage.Question{
		Name:  r.name,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET | classMask,
	})
	b.Question(dnsmessage.Question{
		Name:  r.name,
		Type:  dnsmessage.TypeAAAA,
		Class: dnsmessage.ClassINET | classMask,
	})

	msg, err := b.Finish()
	if err != nil {
		return err
	}

	log.Debug("sending query for %s", r.name)
	if _, err := c.conn4.WriteTo(msg, mdnsGroupAddr4); err != nil {
		return err
	}
	if _, err := c.conn6.WriteTo(msg, mdnsGroupAddr6); err != nil {
		return err
	}
	return nil
}

// Announce a newly generated ephemeral domain name over mDNS.
func (c *Client) Announce(ctx context.Context, name string, ip net.IP, ttl time.Duration) error {
	if c.conn4 == nil || c.conn6 == nil {
		return fmt.Errorf("mDNS client not connected")
	}

	if !isEphemeralLocalDomain(name) {
		return fmt.Errorf("invalid ephemeral domain: %s", name)
	}
	uuid := name[:len(name)-6]

	// Save a record in the cache, to answer incoming queries later.
	c.Lock()
	r := &record{
		name:    dnsmessage.MustNewName(name + "."),
		ip:      ip,
		expires: time.Now().Add(ttl),
		ours:    true,
	}
	c.cache[uuid] = r
	c.Unlock()

	c.maybePruneCache()

	conn, dst := c.conn4, mdnsGroupAddr4
	if ip.To4() == nil {
		conn, dst = c.conn6, mdnsGroupAddr6
	}

	// Send an unsolicited DNS response to the multicast group.
	return c.sendResponse(r, dst, conn)
}

// Resolve the ephemeral mDNS domain to an IP address. Blocks until resolved or
// until the context is done.
func (c *Client) Resolve(ctx context.Context, name string) (net.IP, error) {
	if c.conn4 == nil || c.conn6 == nil {
		return nil, fmt.Errorf("mDNS client not connected")
	}

	if !isEphemeralLocalDomain(name) {
		return nil, fmt.Errorf("invalid ephemeral domain: %s", name)
	}
	uuid := name[:len(name)-6] // strip ".local" suffix

	c.Lock()
	r := c.cache[uuid]
	if r == nil {
		// Construct a new unresolved record.
		r = &record{
			name:    dnsmessage.MustNewName(name + "."),
			expires: time.Now().Add(2 * time.Minute),
			ready:   new(uint32),
			readyCh: make(chan struct{}),
		}
		c.cache[uuid] = r
	}
	c.Unlock()

	c.maybePruneCache()

	return c.waitUntilResolved(ctx, r)
}

// Block until the given record is resolved. Re-send mDNS query rep
func (c *Client) waitUntilResolved(ctx context.Context, r *record) (net.IP, error) {
	if r.ip != nil {
		return r.ip, nil
	}

	// Re-send the query repeatedly until we either get an answer or time out.
	wait := initialQueryInterval
	timer := time.NewTimer(wait)
	defer timer.Stop()

	for {
		if err := c.sendQuery(r); err != nil {
			return nil, err
		}

		// Wait for pending query to resolve.
		select {
		case <-timer.C:
			wait *= 2 // exponential backoff
			timer.Reset(wait)
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to resolve %s: %w", r.name, ctx.Err())
		case <-r.readyCh:
			log.Debug("resolved %s to %s", r.name, r.ip)
			return r.ip, nil
		}
	}
}

// Prune expired records from the cache, if it has grown too large. (The goal is
// just to prevent the cache from growing unboundedly, we don't actually need to
// prune very often.)
func (c *Client) maybePruneCache() {
	if len(c.cache) > c.pruneSize {
		go c.doPruneCache()
	}
}

func (c *Client) doPruneCache() {
	c.Lock()
	defer c.Unlock()

	now := time.Now()
	for key, r := range c.cache {
		if now.After(r.expires) {
			delete(c.cache, key)
		}
	}

	// Reset the prune size based on the current number of non-expired records.
	c.pruneSize = len(c.cache) + initialPruneSize
}

// Check if the given address is an ephemeral mDNS hostname.
func isEphemeralLocalDomain(host string) bool {
	// Per https://tools.ietf.org/html/draft-ietf-rtcweb-mdns-ice-candidates-04#section-3.1.1,
	// an ephemeral hostname should consist of a version 4 UUID followed by
	// ".local". We check for the latter and make a rough guess for the rest.
	return strings.HasSuffix(host, ".local") && strings.Count(host, ".") == 1 && len(host) >= 36+6
}
