package ice

import (
	"net"
	"testing"
)

func TestSortInPriorityOrder(t *testing.T) {
	// Three candidate pairs, each with different addresses, initially *not* in
	// priority order (100, 99, 101).
	pairs := []*CandidatePair{
		newCandidatePair(1, cand(100, "1.1.1.1", 1000), cand(100, "1.1.1.1", 1001)),
		newCandidatePair(2, cand(99, "2.2.2.2", 2000), cand(99, "2.2.2.2", 2001)),
		newCandidatePair(3, cand(101, "3.3.3.3", 3000), cand(101, "3.3.3.3", 3001)),
	}

	pairs = sortAndPrune(pairs)
	if len(pairs) != 3 {
		t.Errorf("Pairs should not have been pruned: %+v", pairs)
	}

	// After sorting, the highest priority should be first.
	if pairs[0].local.priority != 101 || pairs[1].local.priority != 100 || pairs[2].local.priority != 99 {
		t.Errorf("Pairs are not sorted: %+v", pairs)
	}
}

func TestPruneRedundant(t *testing.T) {
	// Host candidate and server-reflexive candidate with the same base.
	hostCand := cand(100, "1.1.1.1", 1000)
	hostCand.base = &Base{address: hostCand.address}
	srflxCand := cand(99, "1.2.3.4", 1234)
	srflxCand.base = hostCand.base

	// Two candidate pairs with the same local base and same remote address,
	// but different priorities.
	pairs := []*CandidatePair{
		newCandidatePair(1, hostCand, cand(100, "5.5.5.5", 5555)),
		newCandidatePair(2, srflxCand, cand(99, "5.5.5.5", 5555)),
	}

	pairs = sortAndPrune(pairs)
	if len(pairs) != 1 {
		t.Errorf("Pairs should have been pruned: %+v", pairs)
	}
	if pairs[0].local.priority != 100 {
		t.Errorf("Should have selected the higher priority pair: %+v", pairs[0])
	}
}

func TestPruneSkipsInProgress(t *testing.T) {
	// Host candidate and server-reflexive candidate with the same base.
	hostCand := cand(100, "1.1.1.1", 1000)
	hostCand.base = &Base{address: hostCand.address}
	srflxCand := cand(99, "1.2.3.4", 1234)
	srflxCand.base = hostCand.base

	// Two redundant candidate pairs, but the lower priority one is in-progress.
	pairs := []*CandidatePair{
		newCandidatePair(1, hostCand, cand(100, "5.5.5.5", 5555)),
		newCandidatePair(2, srflxCand, cand(99, "5.5.5.5", 5555)),
	}
	pairs[1].state = InProgress

	pairs = sortAndPrune(pairs)
	if len(pairs) != 2 {
		t.Errorf("In-progress pair should not have been pruned: %+v", pairs)
	}
}

// cand returns a Candidate with a specified priority and IP address. Not all
// Candidate fields are populated.
func cand(priority uint32, ip string, port int) Candidate {
	c := Candidate{}
	c.priority = priority
	c.address.protocol = "udp"
	c.address.port = port
	copy(c.address.ip[:], net.ParseIP(ip).To16())
	return c
}
