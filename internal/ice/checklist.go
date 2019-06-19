package ice

import (
	"context"
	"net"
	"sort"
	"sync"
	"time"
)

type Checklist struct {
	state       checklistState

	// Checklist state listeners, each with a unique id.
	listeners      map[int]chan checklistState
	nextListenerID int

	// ICE credentials
	username       string
	localPassword  string
	remotePassword string

	// ID for next candidate pair to be added
	nextPairID int

	pairs []*CandidatePair

	triggeredQueue []*CandidatePair

	// Valid list
	valid []*CandidatePair

	// Selected candidate pair
	selected *CandidatePair

	// Mutex to prevent reading from pairs while they're being modified.
	mutex sync.Mutex

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
				cl.pairs = append(cl.pairs, p)
			}
		}
	}

	cl.pairs = sortAndPrune(cl.pairs)

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

// sortAndPrune sorts the candidate pairs from highest to lowest priority, then
// prunes any redundant pairs.
func sortAndPrune(pairs []*CandidatePair) []*CandidatePair {
	// [RFC8445 §6.1.2.3] Sort pairs from highest to lowest priority.
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Priority() > pairs[j].Priority()
	})

	// [RFC8445 §6.1.2.4] Prune redundant pairs.
	for i := 0; i < len(pairs); i++ {
		p := pairs[i]
		// [draft-ietf-ice-trickle-21 §10] Preserve pairs for which checks are in flight.
		switch p.state {
		case InProgress, Succeeded, Failed:
			continue
		}
		// Compare this pair against higher priority pairs, and remove if redundant.
		for j := 0; j < i; j++ {
			if isRedundant(p, pairs[j]) {
				log.Debug("Pruning %s in favor of %s", p.id, pairs[j].id)
				pairs = append(pairs[:i], pairs[i+1:]...)
				break
			}
		}
	}

	return pairs
}

// [RFC8445 §6.1.2.4] Two candidate pairs are redundant if they have the same
// remote candidate and same local base.
func isRedundant(p1, p2 *CandidatePair) bool {
	return p1.remote.address == p2.remote.address && p1.local.base.address == p2.local.base.address
}

func (cl *Checklist) run(ctx context.Context) {
	lid, stateCh := cl.addListener()
	defer cl.removeListener(lid)

	go func() {
		// Timer for periodic connectivity checks. This is stopped once a
		// candidate pair has been selected.
		Ta := time.NewTicker(50 * time.Millisecond)
		defer Ta.Stop()

		// Timer for keepalives.
		Tr := time.NewTicker(30 * time.Second)
		defer Tr.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case newState := <-stateCh:
				// Checklist state has changed.
				log.Debug("Checklist state: %d", newState)
				switch newState {
				case checklistCompleted:
					// TODO; Just end the run loop when the checklist completes.
					Ta.Stop()
				case checklistFailed:
					log.Fatal("Failed to connect to remote peer")
				}

			case <-Ta.C:
				// [RFC8445 §6.1.4.2] Periodic connectivity check.
				if p := cl.nextPair(); p != nil {
					log.Debug("Next candidate pair to check: %s\n", p)
					if err := cl.sendCheck(p); err != nil {
						log.Warn("Failed to send connectivity check: %s", err)
					}
				}

			case <-Tr.C:
				// [RFC8445 §11] Send STUN binding indication to selected pair.
				if p := cl.selected; p != nil {
					p.sendStun(newStunBindingIndication(), nil)
				}
			}
		}
	}()
}

func (cl *Checklist) getSelected(ctx context.Context) (*CandidatePair, error) {
	lid, stateCh := cl.addListener()
	defer cl.removeListener(lid)

	for {
		if cl.selected != nil {
			return cl.selected, nil
		}

		select {
		case <-stateCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// [RFC8445 §7.3] Respond to STUN binding request by sending a success response.
func (cl *Checklist) handleStunRequest(req *stunMessage, raddr net.Addr, base *Base) {
	p := cl.findPair(base, raddr)
	if p == nil {
		p = cl.adoptPeerReflexiveCandidate(base, raddr, req.getPriority())
	}
	if req.hasUseCandidate() && !p.nominated {
		log.Debug("Nominating %s\n", p.id)
		cl.nominate(p)
	}

	resp := newStunBindingResponse(req.transactionID, raddr, cl.localPassword)
	log.Debug("Sending response %s -> %s: %s\n", base.LocalAddr(), raddr, resp)
	if err := base.sendStun(resp, raddr, nil); err != nil {
		log.Warn("Failed to send STUN response: %s", err)
	}

	cl.triggerCheck(p)
}

// [RFC8445 §7.3.1.3-4] Create a peer reflexive candidate and pair with the base.
func (cl *Checklist) adoptPeerReflexiveCandidate(base *Base, raddr net.Addr, priority uint32) *CandidatePair {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	local := makeHostCandidate(base)
	remote := makePeerReflexiveCandidate(base, raddr, priority)
	log.Debug("New peer-reflexive %s", remote)

	p := newCandidatePair(cl.nextPairID, local, remote)
	p.state = Waiting
	cl.pairs = append(cl.pairs, p)
	cl.nextPairID++

	cl.pairs = sortAndPrune(cl.pairs)
	return p
}

// Return the next candidate pair to check for connectivity.
func (cl *Checklist) nextPair() *CandidatePair {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if len(cl.triggeredQueue) > 0 {
		p := cl.triggeredQueue[0]
		cl.triggeredQueue = cl.triggeredQueue[1:]
		return p
	}

	// Find the next pair in the Waiting state.
	n := len(cl.pairs)
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

func (cl *Checklist) sendCheck(p *CandidatePair) error {
	req := newStunBindingRequest("")
	req.addAttribute(stunAttrUsername, []byte(cl.username))
	req.addAttribute(stunAttrIceControlled, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	req.addPriority(p.local.peerPriority())
	req.addMessageIntegrity(cl.remotePassword)
	req.addFingerprint()
	p.state = InProgress
	retransmit := time.AfterFunc(cl.rto(), func() {
		// If we don't get a response within the RTO, then move the pair back to Waiting.
		p.state = Waiting
	})

	log.Debug("%s: Sending to %s from %s: %s\n", p.id, p.remote.address, p.local.address, req)
	return p.sendStun(req, func(resp *stunMessage, raddr net.Addr, base *Base) {
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
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	if cl.state != checklistRunning {
		return
	}

	for _, p := range cl.valid {
		if p.nominated {
			log.Info("Selected %s", p)
			cl.selected = p
			cl.state = checklistCompleted
			break
		}
	}

	// TODO: Handle checklist failure

	// Notify listeners that the state has changed.
	for _, ch := range cl.listeners {
		select {
		case ch <- cl.state:
		default:
		}
	}
}

func (cl *Checklist) addListener() (int, <-chan checklistState) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	id := cl.nextListenerID
	ch := make(chan checklistState, 1)
	if cl.listeners == nil {
		cl.listeners = make(map[int]chan checklistState)
	}
	cl.listeners[id] = ch
	cl.nextListenerID++
	return id, ch
}

func (cl *Checklist) removeListener(id int) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	delete(cl.listeners, id)
}

// findPair returns first candidate pair matching the base and remote address
func (cl *Checklist) findPair(base *Base, raddr net.Addr) *CandidatePair {
	remoteAddress := makeTransportAddress(raddr)

	for _, p := range cl.pairs {
		if p.local.address == base.address && p.remote.address == remoteAddress {
			return p
		}
	}

	return nil
}

func (cl *Checklist) triggerCheck(p *CandidatePair) {
	if p.state == Frozen || p.state == Waiting {
		cl.mutex.Lock()
		cl.triggeredQueue = append(cl.triggeredQueue, p)
		cl.mutex.Unlock()
	}
}
