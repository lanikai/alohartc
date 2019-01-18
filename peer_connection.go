package webrtc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/lanikailabs/webrtc/internal/dtls"
	"github.com/lanikailabs/webrtc/internal/ice"
	"github.com/lanikailabs/webrtc/internal/srtp"
)

const (
	nalTypeSingleTimeAggregationPacketA = 24
	nalReferenceIndicatorPriority1      = 1 << 5
	nalReferenceIndicatorPriority2      = 2 << 5
	nalReferenceIndicatorPriority3      = 3 << 5

	naluBufferSize = 2 * 1024 * 1024
)

type PeerConnection struct {
	// Local session description
	localDescription SessionDesc

	// Remote peer session description
	remoteDescription SessionDesc

	// RTP payload type (negotiated via SDP)
	DynamicType uint8

	// SRTP session, established after successful call to Connect()
	srtpSession *srtp.Conn

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
func (pc *PeerConnection) Connect(lcand chan<- string, rcand <-chan string) error {
	remoteUfrag := pc.remoteDescription.GetMedia().GetAttr("ice-ufrag")
	localUfrag := pc.localDescription.GetMedia().GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := pc.localDescription.GetMedia().GetAttr("ice-pwd")
	remotePassword := pc.remoteDescription.GetMedia().GetAttr("ice-pwd")
	ia := ice.NewAgent(username, localPassword, remotePassword)

	// Process incoming remote ICE candidates.
	go func() {
		for c := range rcand {
			err := ia.AddRemoteCandidate(c)
			if err != nil {
				log.Printf("Failed to add remote candidate \"%s\": %s\n", c, err)
			}
		}
		// Signal end of remote candidates.
		ia.AddRemoteCandidate("")
	}()

	conn, err := ia.EstablishConnection(lcand)
	if err != nil {
		return err
	}

	// Load dynamically generated certificate
	cert, err := dtls.X509KeyPair(pc.certPEMBlock, pc.keyPEMBlock)
	if err != nil {
		return err
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
		return err
	}

	pc.srtpSession, err = srtp.NewSession(conn, pc.DynamicType, dc.ClientKey, dc.ClientIV)
	return err
}

func (pc *PeerConnection) Close() {
	if pc.srtpSession != nil {
		pc.srtpSession.Close()
	}
}

// Stream a raw H.264 video over the peer connection. If wholeNALUs is true, assume that each Read()
// returns a whole number of NAL units (this is just an optimization).
func (pc *PeerConnection) StreamH264(source io.Reader, wholeNALUs bool) error {
	if pc.srtpSession == nil {
		return errors.New("Must establish connection before streaming video")
	}

	// Custom splitter. Extracts NAL units.
	ScanNALU := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		i := bytes.Index(data, []byte{0, 0, 1})

		switch i {
		case -1:
			if wholeNALUs {
				// Assume entire remaining data chunk is one NALU.
				return len(data), data, nil
			} else {
				// No NALU boundary found. Wait for more data.
				return 0, nil, nil
			}
		case 0:
			return 3, nil, nil
		case 1:
			return 4, nil, nil
		default:
			return i + 3, data[0:i], nil
		}
	}

	buffer := make([]byte, naluBufferSize)
	scanner := bufio.NewScanner(source)
	scanner.Buffer(buffer, naluBufferSize)
	scanner.Split(ScanNALU)
	var stap []byte
	for scanner.Scan() {
		b := scanner.Bytes()

		// https://tools.ietf.org/html/rfc6184#section-1.3
		fbit := (b[0] & 0x80) >> 7
		nri := (b[0] & 0x60) >> 5
		typ := b[0] & 0x1f
		//log.Printf("F: %b, NRI: %02b, Type: %d, Length: %d\n", fbit, nri, typ, len(b))

		if (typ == 6) || (typ == 7) || (typ == 8) {
			// Wrap SPS/PPS/SEI in STAP-A packet
			// https://tools.ietf.org/html/rfc6184#section-5.7
			if stap == nil {
				stap = []byte{nalTypeSingleTimeAggregationPacketA}
			}
			len := len(b)
			stap = append(stap, byte(len>>8), byte(len))
			stap = append(stap, b...)

			// STAP-A F bit equals the bitwise OR of all aggregated F bits.
			stap[0] |= fbit << 7

			// STAP-A NRI value is the maximum of all aggregated NRI values.
			stapnri := (stap[0] & 0x60) >> 5
			if nri > stapnri {
				stap[0] = (stap[0] &^ 0x60) | (nri << 5)
			}
		} else {
			if stap != nil {
				pc.srtpSession.Stap(stap)
				stap = nil
			}

			// Make a copy of the NALU, since the RTP payload gets encrypted in place.
			nalu := make([]byte, len(b))
			copy(nalu, b)
			pc.srtpSession.Send(nalu)
		}
	}

	return scanner.Err()
}
