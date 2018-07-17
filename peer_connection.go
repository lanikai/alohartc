package webrtc

import (
	"bufio"
	"log"
	"net"
	"strings"
)

type PeerConnection struct {
	// Connection to peer. May be over TCP or UDP.
	conn net.Conn

	// Local session description
	localDescription string

	// Remote peer session description
	remoteDescription string

	password string
}

func NewPeerConnection() *PeerConnection {
	pc := &PeerConnection{}

	return pc
}

// Add remote ICE candidate
func (pc *PeerConnection) AddIceCandidate(candidate string) error {
	// STUN binding request
	StunBindingRequest(candidate, pc.password)

	// STUN binding success response
//	log.Println("send stun binding request")

	// DTLS client hello
//	log.Println("send stun binding request")

	return nil
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() (string, error) {
	return "\r\n", nil
}

// Set remote SDP offer
func (pc *PeerConnection) SetRemoteDescription(sdp string) error {
	pc.remoteDescription = sdp

	scanner := bufio.NewScanner(strings.NewReader(sdp))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "a=ice-pwd") {
			pc.password = strings.Split(line, ":")[1]
			log.Println(pc.password)
		}
	}
	return nil
}
