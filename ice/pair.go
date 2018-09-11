package ice

import (
	"net"
)

type CandidatePair struct {
	conn *net.UDPConn

	local  Candidate
	remote Candidate

	laddr net.Addr
	raddr net.Addr

//	isDefault   bool
//	isValid     bool
//	isNominated bool
//	state       cpState
}

// Candidate pair states
type cpState int
const (
	cpWaiting    cpState = 0
	cpInProgress         = 1
	cpSucceeded          = 2
	cpFailed             = 3
	cpFrozen             = 4
)


func newCandidatePair(conn *net.UDPConn, local, remote Candidate, laddr, raddr net.Addr) *CandidatePair {
	return &CandidatePair{conn: conn, local: local, remote: remote, laddr: laddr, raddr: raddr}
}

func (cp *CandidatePair) String() string {
	ls := cp.local.protocol + ":" + cp.laddr.String()
	rs := cp.remote.protocol + ":" + cp.raddr.String()
	return ls + " -> " + rs
}


func (cp *CandidatePair) Priority() uint64 {
	G := uint64(cp.remote.priority)
	D := uint64(cp.local.priority)
	var B uint64 = 0
	if G > D {
		B = 1
	}
	return min(G, D)<<32 + max(G, D)<<1 + B
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
