package signaling

import (
	"context"
	"sync"

	"github.com/lanikai/alohartc/internal/ice"
)

// A SessionHandler responds to incoming calls.
type SessionHandler func(s *Session)

// A Session represents a sequence of interactions with the signaling server,
// wherein two peers attempt to establish a direct connection. It includes the
// SDP offer/answer and ICE candidate exchange.
type Session struct {
	// Context used to indicate the end of the session.
	context.Context

	// Channel for receiving SDP offer from remote peer.
	Offer <-chan string

	// Channel for receiving remote ICE candidates.
	RemoteCandidates <-chan ice.Candidate

	// Client-specific function for sending SDP answer to remote peer.
	SendAnswer func(sdpAnswer string) error

	// Client-specific function for sending local ICE candidates to remote peer.
	SendLocalCandidate func(c *ice.Candidate) error

	// TODO: Add method to close session.
}

// A ListenFunc connects to a signaling server and listens for incoming calls.
// For each call it creates a Session object and invokes the provided handler.
type ListenFunc func(handler SessionHandler) error

var listeners []ListenFunc

func RegisterListener(lf ListenFunc) {
	listeners = append(listeners, lf)
}

// Listen invokes all registered listeners, passing each new Session to the
// provided handler. Blocks until all listeners have returned.
func Listen(h SessionHandler) {
	if len(listeners) == 0 {
		log.Warn("No signaling listeners registered.")
		return
	}

	// Start listeners, then wait until they're all finished.
	var wg sync.WaitGroup
	for _, l := range listeners {
		wg.Add(1)
		go func(listen ListenFunc) {
			err := listen(h)
			if err != nil {
				log.Warn("Signaling listener failed: %v", err)
			}
			wg.Done()
		}(l)
	}
	wg.Wait()
}
