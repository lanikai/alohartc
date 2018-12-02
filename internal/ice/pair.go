package ice

import (
	"fmt"
	"log"
)

type CandidatePair struct {
	id         string
	local      Candidate
	remote     Candidate
	foundation string
	component  int

	state     CandidatePairState
	nominated bool
}

// Candidate pair states
type CandidatePairState int

const (
	Frozen     CandidatePairState = 0
	Waiting                       = 1
	InProgress                    = 2
	Succeeded                     = 3
	Failed                        = 4
)

func newCandidatePair(seq int, local, remote Candidate) *CandidatePair {
	if local.component != remote.component {
		log.Panicf("Candidates in pair have different components: %d != %d", local.component, remote.component)
	}
	id := fmt.Sprintf("Pair#%d", seq)
	foundation := fmt.Sprintf("%s/%s", local.foundation, remote.foundation)
	return &CandidatePair{id: id, local: local, remote: remote, foundation: foundation, component: local.component}
}

func (p *CandidatePair) String() string {
	var state string
	switch p.state {
	case Frozen:
		state = "Frozen"
	case Waiting:
		state = "Waiting"
	case InProgress:
		state = "In Progress"
	case Succeeded:
		state = "Succedeed"
	case Failed:
		state = "Failed"
	}
	return fmt.Sprintf("%s: %s -> %s [%s]", p.id, p.local.address, p.remote.address, state)
}

// TODO: Handle case where we're the controlling agent.
func (p *CandidatePair) Priority() uint64 {
	G := uint64(p.remote.priority)
	D := uint64(p.local.priority)
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
