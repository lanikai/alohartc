package signaling

// TODO: Rename Client -> Signaler.
type Client interface {
	// Connect to the signaling server and handle incoming sessions.
	//
	// Blocks until an error occurs or until the client is explicitly shut down.
	Listen() error

	// Interrupt the signaling client.
	Shutdown() error
}

type SessionHandler func(s *Session)

func NewClient(handler SessionHandler) (Client, error) {
	// TODO: Support pluggable signaling clients, using conditional compilation.
	//return newLocalWebClient(handler)
	return newMQTTSignaler(handler)
}