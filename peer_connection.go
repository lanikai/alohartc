package webrtc

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/thinkski/webrtc/internal/srtp"
)

type PeerConnection struct {
	// Connection to peer. May be over TCP or UDP.
	conn *net.UDPConn

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
	fields := strings.Fields(candidate)
	if protocol := fields[2]; protocol != "udp" {
		// Skip non-UDP
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

	// Load client certificate from file
	//	cert, err := dtls.LoadX509KeyPair("client.pem", "client.key")
	//	if err != nil {
	//		log.Fatal(err)
	//	}

	// Send DTLS client hello
	//	_, err = dtls.NewSession(
	//		pc.conn,
	//		&dtls.Config{
	//			Certificates:           []dtls.Certificate{cert},
	//			InsecureSkipVerify:     true,
	//			Renegotiation:          dtls.RenegotiateFreelyAsClient,
	//			SessionTicketsDisabled: false,
	//			ClientSessionCache:     dtls.NewLRUClientSessionCache(-1),
	//			ProtectionProfiles:     []uint16{dtls.SRTP_AES128_CM_HMAC_SHA1_80},
	//		},
	//	)
	//	if err != nil {
	//		log.Println(err)
	//	}

	// Send SRTP stream
	srtpSession, err := srtp.NewSession(pc.conn)
	defer srtpSession.Close()

	// Open file with H.264 test data
	h264file, err := os.Open("testdata/pi.264")
	if err != nil {
		log.Fatal(err)
	}

	// Custom splitter. Extracts NAL units.
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		a := bytes.Index(data, []byte{0, 0, 0, 1})
		if a == -1 {
			return len(data), nil, nil
		} else {
			z := bytes.Index(data[a+4:], []byte{0, 0, 0, 1})
			if z == -1 {
				return a, nil, nil
			} else {
				return z + 4, data[a+4 : z-1], nil
			}
		}
	}

	// Open H.264 file. Send each frame as RTP packet (or set of packets with same timestamp)
	scanner := bufio.NewScanner(h264file)
	scanner.Split(split)
	for scanner.Scan() {
		srtpSession.Send(scanner.Bytes())
	}

	return nil
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() (string, error) {
	tmpl := `v=0
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
a=fingerprint:sha-256 B8:D4:15:07:0A:E4:6B:6D:67:B9:A1:4F:7D:B8:29:A9:93:25:74:97:91:A4:41:58:68:F3:94:E6:57:A9:5F:BC
a=setup:active
a=mid:video
a=sendonly
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:100 H264/90000
a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42001f
a=ssrc:2541098696 cname:cYhx/N8U7h7+3GW3
a=ssrc:2541098696 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:2541098696 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
a=ssrc:2541098696 label:e9b60276-a415-4a66-8395-28a893918d4c
`
	return tmpl, nil
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
