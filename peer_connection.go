package webrtc

import (
	"log"
)

type PeerConnection struct {
}

func NewPeerConnection() *PeerConnection {
	pc := &PeerConnection{}

	return pc
}

// Add remote ICE candidate
func (pc *PeerConnection) AddIceCandidate(candidate string) error {
	// STUN binding request
	log.Println("send stun binding request")

	// STUN binding success response
	log.Println("send stun binding request")

	// DTLS client hello
	log.Println("send stun binding request")

	return nil
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() (string, error) {
	return "\r\n", nil
}

// Set remote SDP offer
func (pc *PeerConnection) SetRemoteDescription(sdp string) error {
	return nil
}
