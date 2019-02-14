package alohartc

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/lanikai/alohartc/internal/dtls" // subtree merged pions/dtls
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/mux"
	"github.com/lanikai/alohartc/internal/sdp"
	"github.com/lanikai/alohartc/internal/srtp"
)

const (
	sdpUsername = "lanikai"

	nalTypeSingleTimeAggregationPacketA = 24
	nalReferenceIndicatorPriority1      = 1 << 5
	nalReferenceIndicatorPriority2      = 2 << 5
	nalReferenceIndicatorPriority3      = 3 << 5

	naluBufferSize = 2 * 1024 * 1024

	srtpMasterKeyLen     = 16
	srtpMasterKeySaltLen = 14
)

type PeerConnection struct {
	// Local session description
	localDescription sdp.Session

	// Remote peer session description
	remoteDescription sdp.Session

	// RTP payload type (negotiated via SDP)
	DynamicType uint8

	iceAgent *ice.Agent

	// SRTP session, established after successful call to Connect()
	srtpSession *srtp.Conn

	// Local certificate
	certificate *x509.Certificate // Public key
	privateKey  crypto.PrivateKey // Private key
	fingerprint string

	// channel for signalling teardown
	teardown chan bool

	mux *mux.Mux
}

func NewPeerConnection() *PeerConnection {
	// Dynamically generate a certificate for the peer connection
	certificate, privateKey, err := dtls.GenerateSelfSigned()
	if err != nil {
		panic(err)
	}

	// Compute certificate fingerprint for later inclusion in SDP offer/answer
	fingerprint, err := dtls.Fingerprint(certificate, dtls.HashAlgorithmSHA256)
	if err != nil {
		panic(err)
	}

	// Instantiate a peer connection object
	pc := &PeerConnection{
		iceAgent:    ice.NewAgent(),
		certificate: certificate,
		privateKey:  privateKey,
		fingerprint: "sha-256 " + strings.ToUpper(fingerprint),
		teardown:    make(chan bool),
	}

	return pc
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) createAnswer() sdp.Session {
	s := sdp.Session{
		Version: 0,
		Origin: sdp.Origin{
			Username:       sdpUsername,
			SessionId:      strconv.FormatInt(time.Now().UnixNano(), 10),
			SessionVersion: 2,
			NetworkType:    "IN",
			AddressType:    "IP4",
			Address:        "127.0.0.1",
		},
		Name: "-",
		Time: []sdp.Time{
			{nil, nil},
		},
		Attributes: []sdp.Attribute{
			{"group", pc.remoteDescription.GetAttr("group")},
		},
	}

	for _, remoteMedia := range pc.remoteDescription.Media {
		for _, attr := range remoteMedia.Attributes {
			if attr.Key == "rtpmap" && strings.Contains(attr.Value, "H264/90000") {
				// Choose smallest rtpmap entry
				n, _ := strconv.Atoi(strings.Fields(attr.Value)[0])
				if pc.DynamicType == 0 || uint8(n) < pc.DynamicType {
					pc.DynamicType = uint8(n)
				}
			}
		}
		m := sdp.Media{
			Type:   "video",
			Port:   9,
			Proto:  "UDP/TLS/RTP/SAVPF",
			Format: []string{strconv.Itoa(int(pc.DynamicType))},
			Connection: &sdp.Connection{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     "0.0.0.0",
			},
			Attributes: []sdp.Attribute{
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
		s.Media = append(s.Media, m)
	}

	pc.localDescription = s
	return s
}

// Set remote SDP offer. Return SDP answer.
func (pc *PeerConnection) SetRemoteDescription(sdpOffer string) (sdpAnswer string, err error) {
	offer, err := sdp.ParseSession(sdpOffer)
	if err != nil {
		return
	}
	pc.remoteDescription = offer

	answer := pc.createAnswer()

	remoteUfrag := offer.Media[0].GetAttr("ice-ufrag")
	localUfrag := answer.Media[0].GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := answer.Media[0].GetAttr("ice-pwd")
	remotePassword := offer.Media[0].GetAttr("ice-pwd")
	pc.iceAgent.Configure(username, localPassword, remotePassword)

	return answer.String(), nil
}

// Add remote ICE candidate from an SDP candidate string. An empty string denotes the end of
// remote candidates.
func (pc *PeerConnection) AddRemoteCandidate(desc string) error {
	return pc.iceAgent.AddRemoteCandidate(desc)
}

func (pc *PeerConnection) SdpMid() string {
	return pc.remoteDescription.Media[0].GetAttr("mid")
}

// Attempt to connect to remote peer. Send local ICE candidates to lcand.
func (pc *PeerConnection) Connect(lcand chan<- string) error {
	ia := pc.iceAgent

	iceConn, err := ia.EstablishConnection(lcand)
	if err != nil {
		return err
	}

	// Instantiate a new net.Conn multiplexer
	pc.mux = mux.NewMux(iceConn, 8192)

	// Instantiate a new endpoint for DTLS from multiplexer
	dtlsEndpoint := pc.mux.NewEndpoint(mux.MatchDTLS)

	// Instantiate a new endpoint for SRTP from multiplexer
	srtpEndpoint := pc.mux.NewEndpoint(mux.MatchSRTP)

	// Drop SRTCP
	_ = pc.mux.NewEndpoint(mux.MatchSRTCP)

	// Configuration for DTLS handshake, namely certificate and private key
	config := &dtls.Config{Certificate: pc.certificate, PrivateKey: pc.privateKey}

	// Initiate a DTLS handshake as a client
	dtlsConn, err := dtls.Client(dtlsEndpoint, config)
	if err != nil {
		return err
	}

	// Create SRTP keys from DTLS handshake (see RFC5764 Section 4.2)
	material, err := dtlsConn.ExportKeyingMaterial("EXTRACTOR-dtls_srtp", nil, (srtpMasterKeyLen*2)+(srtpMasterKeySaltLen*2))
	if err != nil {
		return err
	}

	// Keying material consists of:
	//   0                    ..     keylen               - 1: write key
	//       keylen           .. 2 * keylen               - 1: read key (unused)
	//   2 * keylen           .. 2 * keylen +     saltlen - 1: write salt
	//   2 * keylen + saltlen .. 2 * keylen + 2 * saltlen - 1: read salt (unused)
	key := append([]byte{}, material[0:srtpMasterKeyLen]...)
	salt := append([]byte{}, material[2*srtpMasterKeyLen:2*srtpMasterKeyLen+srtpMasterKeySaltLen]...)

	// Instantiate a new SRTP session
	pc.srtpSession, err = srtp.NewSession(srtpEndpoint, pc.DynamicType, key, salt)

	return err
}

func (pc *PeerConnection) Close() {
	log.Info("Closing peer connection")
	pc.teardown <- true
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
		// Found NAL unit boundary
		default:
			if data[i-1] == 0 {
				// 4 byte boundary
				return i + 3, data[0 : i-1], nil
			} else {
				// 3 byte boundary
				return i + 3, data[0:i], nil
			}
		}
	}

	buffer := make([]byte, naluBufferSize)
	scanner := bufio.NewScanner(source)
	scanner.Buffer(buffer, naluBufferSize)
	scanner.Split(ScanNALU)
	var stap []byte
	for scanner.Scan() {

		select {
		case <-pc.teardown:
			return nil

		default:
			b := scanner.Bytes()

			// https://tools.ietf.org/html/rfc6184#section-1.3
			forbiddenBit := (b[0] & 0x80) >> 7
			nri := (b[0] & 0x60) >> 5
			typ := b[0] & 0x1f
			//log.Printf("F: %b, NRI: %02b, Type: %d, Length: %d\n", forbiddenBit, nri, typ, len(b))

			if (typ == 6) || (typ == 7) || (typ == 8) {
				// Wrap SPS/PPS/SEI in STAP-A packet
				// https://tools.ietf.org/html/rfc6184#section-5.7
				if stap == nil {
					stap = []byte{nalTypeSingleTimeAggregationPacketA}
				}
				len := len(b)
				stap = append(stap, byte(len>>8), byte(len))
				stap = append(stap, b...)

				// STAP-A forbidden bit is bitwise-OR of all aggregated forbidden bits
				stap[0] |= forbiddenBit << 7

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
	}

	return scanner.Err()
}
