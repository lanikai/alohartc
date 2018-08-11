package webrtc

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/thinkski/webrtc/dtls"
)

type PeerConnection struct {
	// Connection to peer. May be over TCP or UDP.
	conn *net.UDPConn

	// Local session description
	localDescription string

	// Remote peer session description
	remoteDescription SessionDesc

	password string

	// ICE agent responsible for negotiating peer connection.
	ice *IceAgent
}

func NewPeerConnection() *PeerConnection {
	pc := &PeerConnection{}
	pc.ice = &IceAgent{}

	return pc
}

// Add remote ICE candidate
func (pc *PeerConnection) AddIceCandidate(candidate string) error {
	fields := strings.Fields(candidate)
	if protocol := fields[2]; strings.ToLower(protocol) != "udp" {
		log.Println("Skipping non-UDP protocol:", protocol)
		return nil
	}
	ip, port, _ := fields[4], fields[5], fields[11]

	raddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%s", ip, port))
	if err != nil {
		return err
	}

	pc.conn, err = net.DialUDP("udp", nil, raddr)
	if err != nil {
		return err
	}
	defer pc.conn.Close()

	// STUN binding request
	pc.stunBinding(candidate, pc.password)

	// Send DTLS client hello
	if _, err := dtls.DialWithConnection(pc.conn); err != nil {
		log.Println(err)
		return nil
	}

	return nil
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() (SessionDesc, error) {
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
			format: []string{"96"},
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
//				{"ice-options", "trickle"},
				{"fingerprint", "sha-256 05:67:ED:76:91:C6:58:F3:01:CE:F2:01:6A:04:10:53:C3:B3:9A:74:49:68:18:D5:60:D0:BC:25:1B:95:9C:50"},
				{"setup", "active"},
				{"sendonly", ""},
			},
		}
		s.media = append(s.media, m)
	}

/* Sample:
v=0
o=- 6830938501909068252 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE video
a=msid-semantic: WMS SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
m=video 9 UDP/TLS/RTP/SAVPF 100
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:n3E3
a=ice-pwd:auh7I7RsuhlZQgS2XYLStR05
a=ice-options:trickle
a=fingerprint:sha-256 05:67:ED:76:91:C6:58:F3:01:CE:F2:01:6A:04:10:53:C3:B3:9A:74:49:68:18:D5:60:D0:BC:25:1B:95:9C:50
a=setup:active
a=mid:video
a=sendonly
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:100 H264/90000
a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f
a=ssrc-group:FID 2541098696 3215547008
a=ssrc:2541098696 cname:cYhx/N8U7h7+3GW3
a=ssrc:2541098696 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:2541098696 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
a=ssrc:2541098696 label:e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:3215547008 cname:cYhx/N8U7h7+3GW3
a=ssrc:3215547008 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:3215547008 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
a=ssrc:3215547008 label:e9b60276-a415-4a66-8395-28a893918d4c
*/

	return s, nil
}

// Set remote SDP offer
func (pc *PeerConnection) SetRemoteDescription(sdp string) error {
	session, err := parseSession(sdp)
	if err != nil {
		return err
	}

	pc.remoteDescription = session
	pc.password = session.GetMedia().GetAttr("ice-pwd")
	return nil
}
