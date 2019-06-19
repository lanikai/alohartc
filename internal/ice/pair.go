package ice

import (
	"fmt"
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
	Frozen CandidatePairState = iota
	Waiting
	InProgress
	Succeeded
	Failed
)

func (state CandidatePairState) String() string {
	switch state {
	case Frozen:
		return "Frozen"
	case Waiting:
		return "Waiting"
	case InProgress:
		return "In Progress"
	case Succeeded:
		return "Succeeded"
	case Failed:
		return "Failed"
	default:
		panic(fmt.Sprintf("Invalid CandidatePairState: %d", state))
	}
}

func newCandidatePair(seq int, local, remote Candidate) *CandidatePair {
	if local.component != remote.component {
		log.Panicf("Candidates in pair have different components: %d != %d", local.component, remote.component)
	}
	id := fmt.Sprintf("Pair#%d", seq)
	foundation := fmt.Sprintf("%s/%s", local.foundation, remote.foundation)
	return &CandidatePair{
		id:         id,
		local:      local,
		remote:     remote,
		foundation: foundation,
		component:  local.component,
		state:      Frozen,
	}
}

func (p *CandidatePair) getDataStream() *DataStream {
	return p.local.base.makeDataStream(p.remote.address.netAddr())
}

func (p *CandidatePair) sendStun(msg *stunMessage, handler stunHandler) error {
	return p.local.base.sendStun(msg, p.remote.address.netAddr(), handler)
}

func (p *CandidatePair) String() string {
	return fmt.Sprintf("%s: %s -> %s [%s]", p.id, p.local.address, p.remote.address, p.state)
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
