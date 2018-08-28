package webrtc

import (
	"log"
	"net"

//	"github.com/thinkski/webrtc/dtls"
	"github.com/thinkski/webrtc/ice"
)

type PeerConnection struct {
	// Connection to peer. May be over TCP or UDP.
	conn *net.UDPConn

	// Local session description
	localDescription SessionDesc

	// Remote peer session description
	remoteDescription SessionDesc
}

func NewPeerConnection() *PeerConnection {
	pc := &PeerConnection{}
	return pc
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() string {
	s := SessionDesc{
		version: 0,
		origin: OriginDesc{
			username: "golang",
			sessionId: "123456",
			sessionVersion: 2,
			networkType: "IN",
			addressType: "IP4",
			address: "127.0.0.1",
		},
		name: "-",
		time: []TimeDesc{
			{ nil, nil },
		},
		attributes: []AttributeDesc{
			{ "group", pc.remoteDescription.GetAttr("group") },
		},
	}

	for _, remoteMedia := range pc.remoteDescription.media {
		m := MediaDesc{
			typ: "video",
			port: 9,
			proto: "UDP/TLS/RTP/SAVPF",
			format: []string{"100"},
			connection: &ConnectionDesc{
				networkType: "IN",
				addressType: "IP4",
				address: "0.0.0.0",
			},
			attributes: []AttributeDesc{
				{"mid", remoteMedia.GetAttr("mid")},
				{"rtcp", "9 IN IP4 0.0.0.0"},
				{"ice-ufrag", "n3E3"},
				{"ice-pwd", "auh7I7RsuhlZQgS2XYLStR05"},
				{"ice-options", "trickle"},
				{"fingerprint", "sha-256 05:67:ED:76:91:C6:58:F3:01:CE:F2:01:6A:04:10:53:C3:B3:9A:74:49:68:18:D5:60:D0:BC:25:1B:95:9C:50"},
				{"setup", "active"},
				{"sendonly", ""},
				{"rtcp-mux", ""},
				{"rtpmap", "100 H264/90000"},
			},
		}
		s.media = append(s.media, m)
	}

	pc.localDescription = s
	return s.String()

}

// Set remote SDP offer
func (pc *PeerConnection) SetRemoteDescription(sdp string) error {
	session, err := parseSession(sdp)
	if err != nil {
		return err
	}

	pc.remoteDescription = session
	return nil
}

// Receive remote ICE candidates from rcand. Send local ICE candidates to lcand.
func (pc *PeerConnection) Connect(lcand chan<- string, rcand <-chan string) {
	remoteUfrag := pc.remoteDescription.GetMedia().GetAttr("ice-ufrag")
	localUfrag := pc.localDescription.GetMedia().GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := pc.localDescription.GetMedia().GetAttr("ice-pwd")
	remotePassword := pc.remoteDescription.GetMedia().GetAttr("ice-pwd")
	ia := ice.NewAgent(username, localPassword, remotePassword)

	// Send local ICE candidates.
	localCandidates, err := ia.GatherLocalCandidates()
	if err != nil {
		log.Fatal(err)
	}
	for _, c := range localCandidates {
		lcand <- c.String()
	}

	// Wait for remote ICE candidates.
	for c := range rcand {
		ia.AddRemoteCandidate(c)
	}

	_, err = ia.EstablishConnection()
	if err != nil {
		log.Fatal(err)
	}

//	// Send DTLS client hello
//	if _, err := dtls.DialWithConnection(conn); err != nil {
//		log.Fatal(err)
//	}
}

func (pc *PeerConnection) SdpMid() string {
	return pc.remoteDescription.GetMedia().GetAttr("mid")
}
