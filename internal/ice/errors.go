package ice

import "errors"

// Typed errors
var (
	errReadTimeout        = errors.New("ice: read timeout")
	errSTUNInvalidMessage = errors.New("ice: STUN message is malformed")
)
