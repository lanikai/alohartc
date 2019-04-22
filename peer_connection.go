//////////////////////////////////////////////////////////////////////////////
//
// PeerConnection implements a WebRTC native client modeled after the W3C API.
//
// Copyright (c) 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"context"
	"crypto"
	"crypto/x509"
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

	keyLen  = 16
	saltLen = 14

	maxSRTCPSize = 65536
)

type PeerConnection struct {
	// Local context (for signaling)
	localContext context.Context
	teardown     context.CancelFunc

	// Local session description
	localDescription sdp.Session

	// Remote peer session description
	remoteDescription sdp.Session

	// RTP payload type (negotiated via SDP)
	DynamicType uint8

	iceAgent *ice.Agent

	// Local certificate
	certificate *x509.Certificate // Public key
	privateKey  crypto.PrivateKey // Private key
	fingerprint string

	mux *mux.Mux

	// Media tracks
	localVideoTrack  *Track
	remoteVideoTrack *Track // not implemented
	localAudioTrack  *Track // not implemented
	remoteAudioTrack *Track // not implemented
}

// Must is a helper that wraps a call to a function returning
// (*PeerConnection, error) and panics if the error is non-nil. It is intended
// for use in variable initializations such as
//	var pc = alohartc.Must(alohartc.NewPeerConnection(config))
func Must(pc *PeerConnection, err error) *PeerConnection {
	if err != nil {
		panic(err)
	}
	return pc
}

// NewPeerConnection creates a new peer connection object
func NewPeerConnection(config Config) (*PeerConnection, error) {
	return NewPeerConnectionWithContext(context.Background(), config)
}

// NewPeerConnectionWithContext creates a new peer connection object
func NewPeerConnectionWithContext(
	ctx context.Context,
	config Config,
) (*PeerConnection, error) {
	var err error

	// Create new peer connection (with local audio and video)
	pc := &PeerConnection{
		localVideoTrack: &config.VideoTrack,
		localAudioTrack: &config.AudioTrack,
	}

	// Create cancelable context, derived from upstream context
	pc.localContext, pc.teardown = context.WithCancel(ctx)

	// Create new ICE agent for peer connection
	pc.iceAgent = ice.NewAgent(pc.localContext)

	// Dynamically generate a certificate for the peer connection
	if pc.certificate, pc.privateKey, err = dtls.GenerateSelfSigned(); err != nil {
		return nil, err
	}

	// Compute certificate fingerprint for later inclusion in SDP offer/answer
	if pc.fingerprint, err = dtls.Fingerprint(pc.certificate, dtls.HashAlgorithmSHA256); err != nil {
		return nil, err
	}

	return pc, nil
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
				{"fingerprint", "sha-256 " + strings.ToUpper(pc.fingerprint)},
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

	mid := offer.Media[0].GetAttr("mid")
	remoteUfrag := offer.Media[0].GetAttr("ice-ufrag")
	localUfrag := answer.Media[0].GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := answer.Media[0].GetAttr("ice-pwd")
	remotePassword := offer.Media[0].GetAttr("ice-pwd")
	pc.iceAgent.Configure(mid, username, localPassword, remotePassword)

	return answer.String(), nil
}

// Return a channel for receiving local ICE candidates.
func (pc *PeerConnection) LocalICECandidates() <-chan ice.Candidate {
	return pc.iceAgent.ReceiveLocalCandidates()
}

// Add remote ICE candidate.
func (pc *PeerConnection) AddIceCandidate(c ice.Candidate) error {
	return pc.iceAgent.AddRemoteCandidate(c)
}

// Connect to remote peer. Sends local ICE candidates via signaler.
func (pc *PeerConnection) Connect() error {
	// Connect to remote peer
	ctx, cancel := context.WithTimeout(pc.localContext, 10*time.Second)
	iceConn, err := pc.iceAgent.EstablishConnectionWithContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	// Instantiate a new net.Conn multiplexer
	pc.mux = mux.NewMux(iceConn, 8192)

	// Instantiate a new endpoint for DTLS from multiplexer
	dtlsEndpoint := pc.mux.NewEndpoint(mux.MatchDTLS)

	// Instantiate a new endpoint for SRTP from multiplexer
	srtpEndpoint := pc.mux.NewEndpoint(mux.MatchSRTP)

	// Configuration for DTLS handshake, namely certificate and private key
	config := &dtls.Config{Certificate: pc.certificate, PrivateKey: pc.privateKey}

	// Initiate a DTLS handshake as a client
	dtlsConn, err := dtls.Client(dtlsEndpoint, config)
	if err != nil {
		return err
	}

	// Create SRTP keys from DTLS handshake (see RFC5764 Section 4.2)
	material, err := dtlsConn.ExportKeyingMaterial("EXTRACTOR-dtls_srtp", nil, 2*keyLen+2*saltLen)
	if err != nil {
		return err
	}
	offset := 0
	writeKey := append([]byte{}, material[offset:offset+keyLen]...)
	offset += keyLen
	readKey := append([]byte{}, material[offset:offset+keyLen]...)
	offset += keyLen
	writeSalt := append([]byte{}, material[offset:offset+saltLen]...)
	offset += saltLen
	readSalt := append([]byte{}, material[offset:offset+saltLen]...)

	// Start goroutine for processing incoming SRTCP packets
	go srtcpReaderRunloop(pc.mux, readKey, readSalt)

	// Begin a new SRTP session
	if sess, err := srtp.NewSession(
		srtpEndpoint,
		pc.DynamicType,
		writeKey,
		writeSalt,
	); err != nil {
		return err
	} else {
		// Start a goroutine for sending each video tracks to connected peer
		if pc.localVideoTrack != nil {
			go sendVideoTrack(sess, *pc.localVideoTrack)
		}
	}

	// Block until we're done streaming.
	// TODO: Provide a termination condition.
	done := make(chan struct{})
	<-done

	return nil
}

// Close the peer connection
func (pc *PeerConnection) Close() {
	log.Info("Closing peer connection")

	// Call context cancel function
	pc.teardown()

	// Close connection multiplexer and its endpoints
	if pc.mux != nil {
		pc.mux.Close()
	}
}

// sendVideoTrack transmits the local video track to the remote peer
func sendVideoTrack(conn *srtp.Conn, track Track) {
	switch track.(type) {

	// H.264 video track
	case H264VideoTrack:
		var stap []byte
		nalu := make([]byte, 128*1024)
		gotParameterSet := false

		for {
			// Read next NAL unit from H.264 video track
			if n, err := track.Read(nalu); err != nil {
				switch err {
				// End-of-track
				case io.EOF:
					return

				// Should never get here
				default:
					panic(err)
				}

			} else {
				// See https://tools.ietf.org/html/rfc6184#section-1.3
				forbiddenBit := (nalu[0] & 0x80) >> 7
				nri := (nalu[0] & 0x60) >> 5
				typ := nalu[0] & 0x1f

				// Wrap SPS, PPS, and SEI types into a STAP-A packet
				// See https://tools.ietf.org/html/rfc6184#section-5.7
				if (typ == 6) || (typ == 7) || (typ == 8) {
					if stap == nil {
						stap = []byte{nalTypeSingleTimeAggregationPacketA}
					}
					stap = append(stap, byte(n>>8), byte(n))
					stap = append(stap, nalu[:n]...)

					// STAP-A forbidden bit is bitwise-OR of all forbidden bits
					stap[0] |= forbiddenBit << 7

					// STAP-A NRI value is maximum of all NRI values
					stapnri := (stap[0] & 0x60) >> 5
					if nri > stapnri {
						stap[0] = (stap[0] &^ 0x60) | (nri << 5)
					}

					gotParameterSet = true
				} else {
					// Discard NAL units until parameter set received
					if !gotParameterSet {
						continue
					}

					// Send STAP-A when complete
					if stap != nil {
						conn.Stap(stap)
						stap = nil
					}

					// Send NAL
					conn.Send(nalu[:n])
				}
			}
		}

	default:
		panic("unsupported video track")
	}

	panic("should never get here")
}
