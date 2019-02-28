package signaling

import (
	"context"

	"github.com/lanikai/alohartc/internal/ice"
)

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
	SendLocalCandidate func(c ice.Candidate) error

	// TODO: Add method to close session.
}
