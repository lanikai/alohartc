package ice

import (
	"net"
	"sort"
	"sync"
	"time"
)

type Checklist struct {
	state checklistState
	pairs []*CandidatePair

	nextPairID int

	valid    []*CandidatePair
	selected *CandidatePair

	// Listeners that gets notified every time checklist state changes.
	listeners []chan struct{}

	// Mutex to prevent reading from pairs while they're being modified.
	mutex sync.RWMutex

	// Index of the next candidate pair to be checked
	nextToCheck int
}

type checklistState int

const (
	checklistRunning   checklistState = 0
	checklistCompleted                = 1
	checklistFailed                   = 2
)

// Pair up local candidates with remote candidates, and add them to the checklist. Then re-sort and
// re-prune, and unfreeze top candidate pairs.
func (cl *Checklist) addCandidatePairs(locals, remotes []Candidate) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	for _, local := range locals {
		for _, remote := range remotes {
			if canBePaired(local, remote) {
				p := newCandidatePair(cl.nextPairID, local, remote)
				cl.nextPairID++
				log.Debug("Adding candidate pair %s", p)
				// TODO: Check that this is a new foundation, otherwise it should stay Frozen.
				p.state = Waiting
				cl.pairs = append(cl.pairs, p)
			}
		}
	}

	// [RFC8445 §6.1.2.3] Order pairs by priority.
	sort.Slice(cl.pairs, func(i, j int) bool {
		return cl.pairs[i].Priority() > cl.pairs[j].Priority()
	})

	// [RFC8445 §6.1.2.4] Prune redundant pairs.
	for i := 0; i < len(cl.pairs); i++ {
		p := cl.pairs[i]
		// [draft-ietf-ice-trickle-21 §10] Preserve pairs for which checks are in flight.
		switch p.state {
		case Frozen, Waiting:
			continue
		}
		// Remove this pair if it is redundant with a higher priority pair.
		for j := 0; j < i; j++ {
			if isRedundant(p, cl.pairs[j]) {
				log.Debug("Pruning %s in favor of %s", p.id, cl.pairs[j].id)
				cl.pairs = append(cl.pairs[:i], cl.pairs[i+1:]...)
				break
			}
		}
	}

	// TODO: Only change the top candidate per foundation.
	for _, p := range cl.pairs {
		if p.state == Frozen {
			p.state = Waiting
		}
	}

}

// Only pair candidates for the same component. Their transport addresses must be compatible.
func canBePaired(local, remote Candidate) bool {
	return local.component == remote.component &&
		local.address.protocol == remote.address.protocol &&
		local.address.family == remote.address.family &&
		local.address.linkLocal == remote.address.linkLocal
}

// [RFC8445 §6.1.2.4] Two candidates are redundant if they have the same remote candidate and same
// local base.
func isRedundant(p1, p2 *CandidatePair) bool {
	return p1.remote.address == p2.remote.address && p1.local.base.address == p2.local.base.address
}

// Return the next candidate pair to check for connectivity.
func (cl *Checklist) nextPair() *CandidatePair {
	cl.mutex.RLock()
	defer cl.mutex.RUnlock()

	n := len(cl.pairs)
	if n == 0 {
		// Nothing to do yet.
		return nil
	}

	// Find the next pair in the Waiting state.
	for i := 0; i < n; i++ {
		k := (cl.nextToCheck + i) % n
		p := cl.pairs[k]
		if p.state == Waiting {
			cl.nextToCheck = (k + 1) % n
			return p
		}
	}

	// Nothing to do.
	return nil
}

func (cl *Checklist) sendCheck(p *CandidatePair, username, password string) error {
	req := newStunBindingRequest("")
	req.addAttribute(stunAttrUsername, []byte(username))
	req.addAttribute(stunAttrIceControlled, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	req.addPriority(p.local.peerPriority())
	req.addMessageIntegrity(password)
	req.addFingerprint()
	p.state = InProgress
	retransmit := time.AfterFunc(cl.rto(), func() {
		// If we don't get a response within the RTO, then move the pair back to Waiting.
		p.state = Waiting
	})
	log.Debug("%s: Sending to %s from %s: %s\n", p.id, p.remote.address, p.local.address, req)
	return p.local.base.sendStun(req, p.remote.address.netAddr(), func(resp *stunMessage, raddr net.Addr, base Base) {
		retransmit.Stop()
		cl.processResponse(p, resp, raddr)
	})
}

// Compute retransmission time.
// https://tools.ietf.org/html/rfc8445#section-14.3
func (cl *Checklist) rto() time.Duration {
	n := 0
	for _, p := range cl.pairs {
		if p.state == Waiting || p.state == InProgress {
			n++
		}
	}
	// TODO: Base this off Ta, which may be less than 50ms.
	return time.Duration(n) * 50 * time.Millisecond
}

func (cl *Checklist) processResponse(p *CandidatePair, resp *stunMessage, raddr net.Addr) {
	if p.state != InProgress {
		log.Debug("Received unexpected STUN response for %s:\n%s\n", p, resp)
		return
	}

	// TODO: Check that the remote address matches, otherwise we have a peer reflexive local
	// candidate.

	switch resp.class {
	case stunSuccessResponse:
		log.Debug("%s: Successful connectivity check", p.id)
		p.state = Succeeded
		cl.mutex.Lock()
		cl.valid = append(cl.valid, p)
		cl.mutex.Unlock()
	case stunErrorResponse:
		p.state = Failed
		// TODO: Retries
	default:
		log.Fatalf("Impossible")
	}

	cl.updateState()
}

func (cl *Checklist) nominate(p *CandidatePair) {
	if p.state == Frozen {
		p.state = Waiting
	}
	p.nominated = true
	cl.updateState()
}

func (cl *Checklist) updateState() {
	if cl.state != checklistRunning {
		return
	}

	cl.mutex.RLock()
	defer cl.mutex.RUnlock()

	for _, p := range cl.valid {
		if p.nominated {
			log.Debug("Selected %s", p)
			cl.selected = p
			cl.state = checklistCompleted
			break
		}
	}

	// TODO: Handle checklist failure

	// Notify listeners.
	for _, listener := range cl.listeners {
		select {
		case listener <- struct{}{}:
		default:
		}
	}
}

func (cl *Checklist) addListener(listener chan struct{}) {
	cl.listeners = append(cl.listeners, listener)
}

func (cl *Checklist) findPair(base Base, raddr net.Addr) *CandidatePair {
	remoteAddress := makeTransportAddress(raddr)
	for _, p := range cl.pairs {
		if p.local.address == base.address && p.remote.address == remoteAddress {
			return p
		}
	}
	return nil
}
