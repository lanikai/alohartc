package ice

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/dns/dnsmessage"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// This module implements the RTCWeb mdns-ice-candidates proposal for using
// ephemeral Multicast DNS hostnames to avoid exposing sensitive IP addresses.
// See https://tools.ietf.org/html/draft-ietf-rtcweb-mdns-ice-candidates-04

// Check if the candidate is an ephemeral mDNS hostname.
func isEphemeralLocalDomain(host string) bool {
	// Per https://tools.ietf.org/html/draft-ietf-rtcweb-mdns-ice-candidates-04#section-3.1.1,
	// an ephemeral hostname should consist of a version 4 UUID followed by
	// ".local". We check for the latter and make a rough guess for the rest.
	return strings.HasSuffix(host, ".local") && strings.Count(host, ".") == 1 && len(host) >= 36+6
}

// Multicast DNS addresses, per RFC 6762.
var mdnsGroupAddr4 = &net.UDPAddr{
	IP:   net.ParseIP("224.0.0.251"),
	Port: 5353,
}
var mdnsGroupAddr6 = &net.UDPAddr{
	IP:   net.ParseIP("ff02::fb"),
	Port: 5353,
}

// High bit of CLASS bit in questions and resource records, repurposed by mDNS.
const classMask = 1 << 15

// A cached mDNS record.
type mdnsRecord struct {
	name    dnsmessage.Name
	ip      net.IP
	expires time.Time
	ours    bool

	// ready and readyCh are used to resolve pending mDNS queries.
	ready   *uint32
	readyCh chan struct{}
}

func (r *mdnsRecord) Type() dnsmessage.Type {
	if r.ip.To4() != nil {
		return dnsmessage.TypeA
	} else {
		return dnsmessage.TypeAAAA
	}
}

func (r *mdnsRecord) WaitUntilResolved(ctx context.Context) (net.IP, error) {
	if r.ip != nil {
		return r.ip, nil
	}

	// Re-send the query repeatedly until we either get an answer or time out.
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := mdnsSendQuery(r); err != nil {
			return nil, err
		}

		// Wait for pending query to resolve.
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return nil, fmt.Errorf("mdns: failed to resolve %s: %w", r.name, ctx.Err())
		case <-r.readyCh:
			log.Debug("mdns: resolved %s to %s", r.name, r.ip)
			return r.ip, nil
		}
	}
}

// Finalize the IP address for this record, after receiving an answer to our
// mDNS query. Atomically update the ready flag and close the channel.
func (r *mdnsRecord) Update(ip net.IP, expires time.Time) {
	r.ip = ip
	r.expires = expires
	if r.ready != nil {
		log.Trace(3, "mdns: updating record %s -> %s (*ready = %d)", r.name, r.ip, *r.ready)
	}
	if r.ready != nil && atomic.AddUint32(r.ready, 1) == 1 && r.readyCh != nil {
		close(r.readyCh)
	}
}

// Global mDNS state.
var _mdns struct {
	// UDP connection bound to IPv4 and IPv6 mDNS multicast addresses.
	conn4 *net.UDPConn
	conn6 *net.UDPConn

	// Indicates a clean shutdown.
	stopped bool

	// Cache of ephemeral .local hostname resolutions. The key is the UUID part
	// of the domain (i.e. without the ".local." suffix).
	cache map[string]*mdnsRecord

	sync.Mutex
}

func mdnsStart() error {
	_mdns.Lock()
	defer _mdns.Unlock()

	// We listen on a wildcard address, otherwise our outgoing queries sent to
	// 224.0.0.251 just get looped back to ourselves.
	//mdnsAddr := &net.UDPAddr{
	//	IP:   net.ParseIP("224.0.0.0"),
	//	Port: 5353,
	//}

	conn4, err := net.ListenMulticastUDP("udp4", nil, mdnsGroupAddr4)
	if err != nil {
		return err
	}
	conn6, err := net.ListenMulticastUDP("udp6", nil, mdnsGroupAddr6)
	if err != nil {
		return err
	}

	pconn4 := ipv4.NewPacketConn(conn4)
	pconn6 := ipv6.NewPacketConn(conn6)
	//if err := pconn.SetControlMessage(ipv4.FlagDst, true); err != nil {
	//	conn.Close()
	//	return err
	//}
	// Multicast loopback is necessary for the case when we're running on the
	// same host as the browser (mostly useful for testing).
	if err := pconn4.SetMulticastLoopback(true); err != nil {
		conn4.Close()
		return err
	}
	if err := pconn6.SetMulticastLoopback(true); err != nil {
		conn6.Close()
		return err
	}

	// Read loop to handle incoming mDNS questions/answers.
	go mdnsReadLoop(conn4)
	go mdnsReadLoop(conn6)

	_mdns.conn4 = conn4
	_mdns.conn6 = conn6
	_mdns.stopped = false
	_mdns.cache = make(map[string]*mdnsRecord)
	return nil
}

func mdnsStop() {
	_mdns.Lock()
	defer _mdns.Unlock()

	_mdns.stopped = true
	_mdns.conn4.Close()
	_mdns.conn4 = nil
	_mdns.conn6.Close()
	_mdns.conn6 = nil
}

func mdnsReadLoop(conn *net.UDPConn) {
	log.Trace(3, "mdns: read loop start (%s)", conn.LocalAddr())
	defer log.Trace(3, "mdns: read loop end (%s)", conn.LocalAddr())

	buf := make([]byte, 1500)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if !_mdns.stopped {
				log.Error("mdns: read error (%s): %v", conn.LocalAddr(), err)
			}
			return
		}

		mdnsHangleMessage(buf[:n], src, conn)
	}
}

func mdnsHangleMessage(msg []byte, src *net.UDPAddr, conn *net.UDPConn) {
	var p dnsmessage.Parser
	hdr, err := p.Start(msg)
	if err != nil {
		log.Warn("mdns: invalid DNS message: %v", err)
		return
	}

	log.Trace(3, "mdns: message header = %+v", hdr)
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
			log.Debug("mdns: invalid question: %v", err)
			break
		}
		log.Debug("mdns: received question: %v", q)

		mdnsHandleQuestion(&q, src, conn)
	}

	// Parse answers.
	for {
		a, err := p.Answer()
		if err == dnsmessage.ErrSectionDone {
			break
		}
		if err != nil {
			log.Debug("mdns: invalid answer: %v", err)
			break
		}
		log.Debug("mdns: received answer: %v", a)

		mdnsHandleAnswer(&a)
	}
}

// Inspect an mDNS query and respond if appropriate.
func mdnsHandleQuestion(q *dnsmessage.Question, src *net.UDPAddr, conn *net.UDPConn) {
	// Extract domain name, excluding the final "." character.
	name := q.Name.String()[:q.Name.Length-1]
	if !isEphemeralLocalDomain(name) {
		return
	}

	// Strip ".local" suffix to get the UUID part of the ephemeral domain.
	uuid := name[:len(name)-6]

	// Check if we possess an authoritative record for this domain.
	r, found := _mdns.cache[uuid]
	if !found || !r.ours || q.Type != r.Type() {
		return
	}

	if time.Now().After(r.expires) {
		// Remove expired record.
		_mdns.Lock()
		delete(_mdns.cache, uuid)
		_mdns.Unlock()
	} else {
		dst := src
		if (q.Class & classMask) == 0 {
			// High bit of QCLASS means a unicast response is requested.
			dst = conn.LocalAddr().(*net.UDPAddr)
		}
		log.Debug("mdns: responding to %v with %v", q, r.ip)
		if err := mdnsSendResponse(r, dst, conn); err != nil {
			log.Warn("mdns: failed to send response: %v", err)
		}
	}
}

func mdnsHandleAnswer(a *dnsmessage.Resource) {
	if a.Header.Class&^classMask != dnsmessage.ClassINET {
		log.Trace(3, "mdns: ignoring answer of class %d", a.Header.Class)
		return
	}

	// We're only interested in mDNS-ICE ephemeral domains.
	name := a.Header.Name.String()[:a.Header.Name.Length-1] // strip final "."
	if !isEphemeralLocalDomain(name) {
		log.Trace(3, "mdns: ignoring answer for %s", name)
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
		log.Trace(3, "mdns: ignoring answer of type %T", a.Body)
		return
	}

	uuid := name[:len(name)-6]
	expires := time.Now().Add(time.Duration(a.Header.TTL) * time.Second)

	_mdns.Lock()
	if r := _mdns.cache[uuid]; r != nil {
		// Received an answer to an earlier query. Update the record and notify
		// any listeners.
		r.Update(ip, expires)
	} else {
		// Doesn't answer any of our current pending queries, but cache it
		// anyway in case we want it later.
		_mdns.cache[uuid] = &mdnsRecord{
			name:    a.Header.Name,
			ip:      ip,
			expires: expires,
			ours:    false,
		}
	}

	for u, r := range _mdns.cache {
		log.Trace(9, "mdns: cache[%s] = %v", u, r)
	}
	_mdns.Unlock()
}

func mdnsSendResponse(r *mdnsRecord, dst *net.UDPAddr, conn *net.UDPConn) error {
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

	log.Debug("mdns: sending response to %s", r.ip)
	if _, err := conn.WriteTo(msg, dst); err != nil {
		return err
	}

	return nil
}

func mdnsSendQuery(r *mdnsRecord) error {
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

	log.Debug("mdns: sending query for %s", r.name)

	// From analyzing mDNS traffic from other utilities, it seems common
	// practice to fire the request multiple times (maybe to account for packet
	// loss?).
	for i := 0; i < 2; i++ {
		if _, err := _mdns.conn4.WriteTo(msg, mdnsGroupAddr4); err != nil {
			return err
		}
		if _, err := _mdns.conn6.WriteTo(msg, mdnsGroupAddr6); err != nil {
			return err
		}
	}

	return nil
}

// Announce a newly generated ephemeral domain name over mDNS.
func mdnsAnnounce(ctx context.Context, name string, ip net.IP, ttl time.Duration) error {
	if !strings.HasSuffix(name, ".local") {
		return fmt.Errorf("mdns: cannot announce domain: %s", name)
	}
	uuid := name[:len(name)-6]

	// Save a record in the cache, to answer incoming queries later.
	_mdns.Lock()
	r := &mdnsRecord{
		name:    dnsmessage.MustNewName(name + "."),
		ip:      ip,
		expires: time.Now().Add(ttl),
		ours:    true,
	}
	_mdns.cache[uuid] = r
	_mdns.Unlock()

	conn := _mdns.conn4
	if ip.To4() == nil {
		conn = _mdns.conn6
	}

	// Send an unsolicited DNS response to the multicast group.
	return mdnsSendResponse(r, conn.LocalAddr().(*net.UDPAddr), conn)
}

// Resolve the ephemeral ICE hostname to an IP address. Blocks until resolved or
// until the context is done.
func mdnsResolve(ctx context.Context, host string) (net.IP, error) {
	if _mdns.conn4 == nil {
		panic("mdns: never started")
	}

	if !isEphemeralLocalDomain(host) {
		return nil, fmt.Errorf("mdns: invalid ephemeral domain: %s", host)
	}
	uuid := host[:len(host)-6] // strip ".local" suffix

	_mdns.Lock()
	r := _mdns.cache[uuid]
	if r == nil {
		// Construct a new unresolved record.
		r = &mdnsRecord{
			name:    dnsmessage.MustNewName(host + "."),
			ready:   new(uint32),
			readyCh: make(chan struct{}),
		}
		_mdns.cache[uuid] = r
	}
	_mdns.Unlock()

	return r.WaitUntilResolved(ctx)
}
