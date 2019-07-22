package ice

import (
	"context"
	"net"
	"sync"
)

// RFC 8445: https://tools.ietf.org/html/rfc8445

// In the language of the above specification, this is a Full implementation of a Controlled ICE
// agent, supporting a single component of a single data stream.
type Agent struct {
	mid       string // media stream ID
	component int    // component (currently always 1)

	localCandidates  []Candidate
	remoteCandidates []Candidate

	checklist Checklist

	dataIn chan []byte

	failure error

	sync.Mutex
}

const (
	// How many incoming packets can be enqueued before dropping data.
	packetQueueLength = 64
)

func NewAgent() *Agent {
	return new(Agent)
}

func (a *Agent) fail(err error) {
	a.Lock()
	defer a.Unlock()

	if a.failure == nil {
		a.failure = err
	}
}

func (a *Agent) Configure(mid, username, localPassword, remotePassword string) {
	a.mid = mid
	a.component = 1
	a.checklist.username = username
	a.checklist.localPassword = localPassword
	a.checklist.remotePassword = remotePassword
}

func (a *Agent) Start(ctx context.Context, rcand <-chan Candidate) <-chan Candidate {
	a.dataIn = make(chan []byte, packetQueueLength)
	lcand := make(chan Candidate, 2)
	go a.connect(ctx, rcand, lcand)
	return lcand
}

// The lcand channel will be closed.
func (a *Agent) connect(ctx context.Context, rcand <-chan Candidate, lcand chan<- Candidate) {
	// Create a base for each network interface.
	bases, err := initializeBases(a.component, a.mid)
	if err != nil {
		close(lcand)
		a.fail(err)
		return
	}

	// Start read loop for each base.
	for _, base := range bases {
		go base.readLoop(a.handleStun, a.dataIn)
	}

	// Process incoming remote candidates.
	go a.addAllRemoteCandidates(ctx, rcand)

	// Gather local candidates for each base.
	go func() {
		defer close(lcand)
		gatherAllCandidates(ctx, bases, func(c Candidate) {
			a.addLocalCandidate(c)
			select {
			case lcand <- c:
			case <-ctx.Done():
			}
		})
	}()

	// Begin connectivity checks.
	a.checklist.run(ctx)
}

// GetDataStream waits for a connection to be established.
func (a *Agent) GetDataStream(ctx context.Context) (*DataStream, error) {
	if a.failure != nil {
		return nil, a.failure
	}

	// Wait for a candidate pair to be selected.
	p, err := a.checklist.getSelected(ctx, nil)
	if err != nil {
		return nil, err
	}

	ds := newDataStream(p, a.dataIn)

	// Keep checking in case the selected pair changes, until ctx is canceled.
	go func() {
		for {
			p, err = a.checklist.getSelected(ctx, p)
			if err != nil {
				return
			}
			ds.update(p)
		}
	}()

	return ds, nil
}

func (a *Agent) addRemoteCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()

	log.Info("Remote ICE %s", c)
	a.remoteCandidates = append(a.remoteCandidates, c)
	// Pair new remote candidate with all existing local candidates.
	a.checklist.addCandidatePairs(a.localCandidates, []Candidate{c})
}

func (a *Agent) addAllRemoteCandidates(ctx context.Context, rcand <-chan Candidate) {
	for {
		select {
		case c, ok := <-rcand:
			if !ok {
				return
			}
			if c.address.protocol == UDP {
				a.addRemoteCandidate(c)
			} else {
				log.Debug("Ignoring non-UDP remote candidate: %s", c)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *Agent) addLocalCandidate(c Candidate) {
	a.Lock()
	defer a.Unlock()

	log.Info("Local ICE %s", c)
	a.localCandidates = append(a.localCandidates, c)
	// Pair new local candidate with all existing remote candidates.
	a.checklist.addCandidatePairs([]Candidate{c}, a.remoteCandidates)
}

func (a *Agent) handleStun(msg *stunMessage, raddr net.Addr, base *Base) {
	allowedMethods := map[uint16]bool{
		stunBindingMethod: true,
		stunSendMethod:    true,
	}
	if !allowedMethods[msg.method] {
		log.Debug("Unexpected STUN message: %s", msg)
		return
	}

	switch msg.class {
	case stunRequest:
		a.checklist.handleStunRequest(msg, raddr, base)
	case stunIndication:
		// No-op
	case stunSuccessResponse, stunErrorResponse:
		log.Debug("Received unexpected STUN response: %s\n", msg)
	}
}
