package signaling

// A signaling Client connects the signaling server and waits for a remote peer
// to initiate a call session.
type Client interface {
	// Listen connects to the signaling server and handles incoming sessions.
	//
	// Blocks until an error occurs or until the client is explicitly shut down.
	Listen() error

	// Shutdown interrupts the signaling client.
	Shutdown() error
}

// NewClient returns a new signaling Client.
var NewClient func(handler SessionHandler) (Client, error)
