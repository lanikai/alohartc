package mdns

import (
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// A cached mDNS record.
type record struct {
	name    dnsmessage.Name
	ip      net.IP
	expires time.Time
	ours    bool

	// ready and readyCh are used to resolve pending mDNS queries.
	ready   *uint32
	readyCh chan struct{}
}

func (r *record) Type() dnsmessage.Type {
	if r.ip.To4() != nil {
		return dnsmessage.TypeA
	} else {
		return dnsmessage.TypeAAAA
	}
}

// Finalize the IP address for this record, after receiving an answer to our
// mDNS query. Atomically update the ready flag and close the channel.
func (r *record) Update(ip net.IP, expires time.Time) {
	r.ip = ip
	r.expires = expires
	if r.ready != nil {
		log.Trace(3, "updating record %s -> %s (*ready = %d)", r.name, r.ip, *r.ready)
	}
	if r.ready != nil && atomic.AddUint32(r.ready, 1) == 1 && r.readyCh != nil {
		close(r.readyCh)
	}
}
