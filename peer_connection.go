package webrtc

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/thinkski/webrtc/internal/dtls"
	"github.com/thinkski/webrtc/internal/ice"
	"github.com/thinkski/webrtc/internal/srtp"
	"github.com/thinkski/webrtc/internal/v4l2"
)

const (
	demoVideoBitrate = 2e6
	demoVideoDevice  = "/dev/video0"
	demoVideoHeight  = 720
	demoVideoWidth   = 1280

	nalTypeSingleTimeAggregationPacketA = 24
	nalReferenceIndicatorPriority1      = 1 << 5
	nalReferenceIndicatorPriority2      = 2 << 5
	nalReferenceIndicatorPriority3      = 3 << 5
)

type PeerConnection struct {
	// Local session description
	localDescription SessionDesc

	// Remote peer session description
	remoteDescription SessionDesc

	// RTP payload type (negotiated via SDP)
	DynamicType uint8

	// Local certificate
	certPEMBlock []byte // Public key
	keyPEMBlock  []byte // Private key
	fingerprint  string
}

func NewPeerConnection() *PeerConnection {
	// Dynamically generate a certificate for the peer connection
	cert, key, fp, err := generateCertificate()
	if err != nil {
		panic(err)
	}

	pc := &PeerConnection{
		certPEMBlock: cert,
		keyPEMBlock:  key,
		fingerprint:  fp,
	}

	return pc
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) CreateAnswer() string {
	s := SessionDesc{
		version: 0,
		origin: OriginDesc{
			username:       "golang",
			sessionId:      "123456",
			sessionVersion: 2,
			networkType:    "IN",
			addressType:    "IP4",
			address:        "127.0.0.1",
		},
		name: "-",
		time: []TimeDesc{
			{nil, nil},
		},
		attributes: []AttributeDesc{
			{"group", pc.remoteDescription.GetAttr("group")},
		},
	}

	for _, remoteMedia := range pc.remoteDescription.media {
		for _, attr := range remoteMedia.attributes {
			if attr.key == "rtpmap" && strings.Contains(attr.value, "H264/90000") {
				// Choose smallest rtpmap entry
				n, _ := strconv.Atoi(strings.Fields(attr.value)[0])
				if pc.DynamicType == 0 || uint8(n) < pc.DynamicType {
					pc.DynamicType = uint8(n)
				}
			}
		}
		m := MediaDesc{
			typ:    "video",
			port:   9,
			proto:  "UDP/TLS/RTP/SAVPF",
			format: []string{strconv.Itoa(int(pc.DynamicType))},
			connection: &ConnectionDesc{
				networkType: "IN",
				addressType: "IP4",
				address:     "0.0.0.0",
			},
			attributes: []AttributeDesc{
				{"mid", remoteMedia.GetAttr("mid")},
				{"rtcp", "9 IN IP4 0.0.0.0"},
				{"ice-ufrag", "n3E3"},
				{"ice-pwd", "auh7I7RsuhlZQgS2XYLStR05"},
				{"ice-options", "trickle"},
				{"fingerprint", pc.fingerprint},
				{"setup", "active"},
				{"sendonly", ""},
				{"rtcp-mux", ""},
				{"rtcp-rsize", ""},
				{"rtpmap", fmt.Sprintf("%d H264/90000", pc.DynamicType)},
				// Chrome offers following profile-level-id values:
				// 42001f (baseline)
				// 42e01f (constrained baseline)
				// 4d0032 (main)
				// 640032 (high)
				{"fmtp", fmt.Sprintf("%d level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", pc.DynamicType)},
				{"ssrc", "2541098696 cname:cYhx/N8U7h7+3GW3"},
				{"ssrc", "2541098696 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c"},
				{"ssrc", "2541098696 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv"},
				{"ssrc", "2541098696 label:e9b60276-a415-4a66-8395-28a893918d4c"},
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

func (pc *PeerConnection) SdpMid() string {
	return pc.remoteDescription.GetMedia().GetAttr("mid")
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
		log.Println("Local ICE", c)
		lcand <- c.String()
	}

	// Wait for remote ICE candidates.
	for c := range rcand {
		ia.AddRemoteCandidate(c)
	}

	conn, err := ia.EstablishConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Load dynamically generated certificate
	cert, err := dtls.X509KeyPair(pc.certPEMBlock, pc.keyPEMBlock)
	if err != nil {
		log.Fatal(err)
	}

	// Send DTLS client hello
	dc, err := dtls.NewSession(
		conn,
		&dtls.Config{
			Certificates:           []dtls.Certificate{cert},
			InsecureSkipVerify:     true,
			Renegotiation:          dtls.RenegotiateFreelyAsClient,
			SessionTicketsDisabled: false,
			ClientSessionCache:     dtls.NewLRUClientSessionCache(-1),
			ProtectionProfiles:     []uint16{dtls.SRTP_AES128_CM_HMAC_SHA1_80},
			KeyLogWriter:           os.Stdout,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Send SRTP stream
	srtpSession, err := srtp.NewSession(conn, pc.DynamicType, dc.ClientKey, dc.ClientIV)
	if err != nil {
		log.Fatal(err)
	}
	defer srtpSession.Close()

	// Open video device
	video, err := v4l2.OpenH264(demoVideoDevice, demoVideoWidth, demoVideoHeight)
	if err != nil {
		log.Fatal(err)
	}

	if err := video.SetBitrate(demoVideoBitrate); err != nil {
		log.Fatal(err)
	}

	// Start video
	if err := video.Start(); err != nil {
		log.Fatal(err)
	}

	buffer := make([]byte, demoVideoBitrate)

	// Wrap SPS/PPS/SEI in STAP-A packet
	n, err := video.Read(buffer)
	if err != nil {
		panic(err)
	}
	b := buffer[4:n]
	nals := bytes.Split(b, []byte{0, 0, 0, 1})
	pkt := []byte{nalTypeSingleTimeAggregationPacketA}
	for _, nal := range nals {
		pkt = append(pkt, []byte{byte(len(nal) >> 8), byte(len(nal))}...)
		pkt = append(pkt, nal...)
	}
	srtpSession.Stap(pkt)
	log.Println(buffer[:n])

	// Write remaining NAL units
	for {
		n, err := video.Read(buffer)
		if err != nil {
			panic(err)
		}
		b := buffer[4:n]
		srtpSession.Send(b)
	}
}
