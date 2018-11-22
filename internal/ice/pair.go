package ice

import (
	"fmt"
)

type CandidatePair struct {
	seq int

	local  Candidate
	remote Candidate

	//	isDefault   bool
	//	isValid     bool
	//	isNominated bool

	state cpState
	cin   chan []byte
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

func newCandidatePair(seq int, local, remote Candidate) *CandidatePair {
	cin := make(chan []byte, 32)
	return &CandidatePair{seq: seq, local: local, remote: remote, cin: cin}
}

func (cp *CandidatePair) String() string {
	laddr := cp.local.Addr()
	raddr := cp.remote.Addr()
	return fmt.Sprintf("CP#%d %s:%s -> %s:%s", cp.seq, laddr.Network(), laddr, raddr.Network(), raddr)
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
