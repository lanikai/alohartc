//////////////////////////////////////////////////////////////////////////////
//
// PeerConnection implements a WebRTC native client modeled after the W3C API.
//
// Copyright (c) 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lanikai/alohartc/internal/dtls" // subtree merged pions/dtls
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/mux"
	"github.com/lanikai/alohartc/internal/rtp"
	"github.com/lanikai/alohartc/internal/sdp"
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

	connectTimeout = 10 * time.Second
)

type PeerConnection struct {
	// Local context (for signaling)
	ctx    context.Context
	cancel context.CancelFunc

	// Local session description
	localDescription sdp.Session

	// Remote peer session description
	remoteDescription sdp.Session

	// RTP payload type (negotiated via SDP)
	DynamicType uint8

	iceAgent         *ice.Agent
	remoteCandidates chan ice.Candidate

	// Callback when a local ICE candidate is available.
	OnIceCandidate func(*ice.Candidate)

	// Local certificate
	certificate *x509.Certificate // Public key
	privateKey  crypto.PrivateKey // Private key
	fingerprint string

	// Media tracks
	localAudio media.AudioSource
	localVideo media.VideoSource
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
func NewPeerConnectionWithContext(ctx context.Context, config Config) (*PeerConnection, error) {
	// Create cancelable context, derived from upstream context
	ctx, cancel := context.WithCancel(ctx)

	// Create new peer connection (with local audio and video)
	pc := &PeerConnection{
		ctx:              ctx,
		cancel:           cancel,
		localAudio:       config.LocalAudio,
		localVideo:       config.LocalVideo,
		iceAgent:         ice.NewAgent(),
		remoteCandidates: make(chan ice.Candidate, 4),

		// Set initial dummy handler for local ICE candidates.
		OnIceCandidate: func(c *ice.Candidate) {
			log.Warn("No OnICECandidate handler: %v", c)
		},
	}

	var err error

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
func (pc *PeerConnection) createAnswer() (sdp.Session, error) {
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

		type payloadTypeAttributes struct {
			nack   bool
			pli    bool
			fmtp   string
			codec  string
			reject bool
		}

		supportedPayloadTypes := make(map[int]*payloadTypeAttributes)

		// Search attributes for supported codecs
		for _, attr := range remoteMedia.Attributes {
			var pt int
			var text string

			// Parse payload type from attribute. Will bin by payload type.
			pt = -1
			switch attr.Key {
			case "rtpmap":
				fallthrough
			case "fmtp":
				fallthrough
			case "rtcp-fb":
				if _, err := fmt.Sscanf(
					attr.Value, "%3d %s", &pt, &text,
				); err != nil {
					log.Warn(fmt.Sprintf("malformed %s", attr.Key))
					break // switch
				}
			}
			if pt < 0 {
				continue // Ignore unsupported attributes
			}

			if _, ok := supportedPayloadTypes[pt]; !ok {
				supportedPayloadTypes[pt] = &payloadTypeAttributes{}
			}
			switch attr.Key {
			case "rtpmap":
				switch text {
				case "H264/90000":
					supportedPayloadTypes[pt].codec = text
				}
			case "rtcp-fb":
				switch text {
				case "nack":
					supportedPayloadTypes[pt].nack = true
				}
			case "fmtp":
				supportedPayloadTypes[pt].fmtp = text
				if !strings.Contains(text, "packetization-mode=1") {
					supportedPayloadTypes[pt].reject = true
				}
				if !strings.Contains(text, "profile-level-id=42e01f") {
					supportedPayloadTypes[pt].reject = true
				}
			}
		}

		// Require 24 and 128 bits of randomness for ufrag and pwd, respectively
		rnd := make([]byte, 3+16)
		if _, err := rand.Read(rnd); err != nil {
			return sdp.Session{}, err
		}

		// Base64 encode ice-ufrag and ice-pwd
		ufrag := base64.StdEncoding.EncodeToString(rnd[0:3])
		pwd := base64.StdEncoding.EncodeToString(rnd[3:])

		// Media description with first part of attributes
		m := sdp.Media{
			Type:  "video",
			Port:  9,
			Proto: "UDP/TLS/RTP/SAVPF",
			Connection: &sdp.Connection{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     "0.0.0.0",
			},
			Attributes: []sdp.Attribute{
				{"mid", remoteMedia.GetAttr("mid")},
				{"rtcp", "9 IN IP4 0.0.0.0"},
				{"ice-ufrag", ufrag},
				{"ice-pwd", pwd},
				{"ice-options", "trickle"},
				{"ice-options", "ice2"},
				{"fingerprint", "sha-256 " + strings.ToUpper(pc.fingerprint)},
				{"setup", "active"},
				{"sendonly", ""},
				{"rtcp-mux", ""},
				{"rtcp-rsize", ""},
			},
		}

		// Additional attributes per payload type
		for pt, a := range supportedPayloadTypes {
			switch {
			case "H264/90000" == a.codec && "" != a.fmtp && !a.reject:
				m.Attributes = append(
					m.Attributes,
					sdp.Attribute{"rtpmap", fmt.Sprintf("%d %s", pt, a.codec)},
				)

				if a.nack {
					m.Attributes = append(
						m.Attributes,
						sdp.Attribute{"rtcp-fb", fmt.Sprintf("%d nack", pt)},
					)
				}

				m.Attributes = append(
					m.Attributes,
					sdp.Attribute{"fmtp", fmt.Sprintf("%d %s", pt, a.fmtp)},
				)
				m.Format = append(m.Format, strconv.Itoa(pt))

				// TODO [chris] Fix payload type selection. Currently we're
				// rejecting essentially all but one payload type, assigned
				// here. However, we should be prepared to receive RTP flows
				// for each accepted payload type.
				pc.DynamicType = uint8(pt)
			}
		}

		// Final attributes
		m.Attributes = append(
			m.Attributes,
			[]sdp.Attribute{
				{"ssrc", "2541098696 cname:cYhx/N8U7h7+3GW3"},
				{"ssrc", "2541098696 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c"},
				{"ssrc", "2541098696 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv"},
				{"ssrc", "2541098696 label:e9b60276-a415-4a66-8395-28a893918d4c"},
			}...,
		)

		s.Media = append(s.Media, m)
	}

	pc.localDescription = s
	return s, nil
}

// Set remote SDP offer. Return SDP answer.
func (pc *PeerConnection) SetRemoteDescription(sdpOffer string) (sdpAnswer string, err error) {
	offer, err := sdp.ParseSession(sdpOffer)
	if err != nil {
		return
	}
	pc.remoteDescription = offer

	answer, err := pc.createAnswer()
	if err != nil {
		return
	}

	mid := offer.Media[0].GetAttr("mid")
	remoteUfrag := offer.Media[0].GetAttr("ice-ufrag")
	localUfrag := answer.Media[0].GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := answer.Media[0].GetAttr("ice-pwd")
	remotePassword := offer.Media[0].GetAttr("ice-pwd")
	pc.iceAgent.Configure(mid, username, localPassword, remotePassword)

	// ICE gathering begins implicitly after offer/answer exchange.
	go pc.startGathering()

	return answer.String(), nil
}

func (pc *PeerConnection) startGathering() {
	log.Debug("Starting ICE gathering")
	lcand := pc.iceAgent.Start(pc.ctx, pc.remoteCandidates)
	for {
		select {
		case c, more := <-lcand:
			if !more {
				// Signal end-of-candidates.
				pc.OnIceCandidate(nil)
				return
			}
			pc.OnIceCandidate(&c)
		case <-pc.ctx.Done():
			return
		}
	}
}

// AddIceCandidate adds a remote ICE candidate.
func (pc *PeerConnection) AddIceCandidate(c *ice.Candidate) {
	if c == nil {
		// nil means end-of-candidates.
		close(pc.remoteCandidates)
		pc.remoteCandidates = nil
	} else {
		select {
		case pc.remoteCandidates <- *c:
		case <-pc.ctx.Done():
		}
	}
}

// Stream establishes a connection to the remote peer, and streams media to/from
// the configured tracks. Blocks until an error occurs, or until the
// PeerConnection is closed.
func (pc *PeerConnection) Stream() error {
	// Wait for ICE agent to establish a connection.
	timeoutCtx, _ := context.WithTimeout(pc.ctx, connectTimeout)
	dataStream, err := pc.iceAgent.GetDataStream(timeoutCtx)
	if err != nil {
		return err
	}
	defer dataStream.Close()

	// Instantiate a new net.Conn multiplexer
	dataMux := mux.NewMux(dataStream, 8192)
	defer dataMux.Close()

	// Instantiate a new endpoint for DTLS from multiplexer
	dtlsEndpoint := dataMux.NewEndpoint(mux.MatchDTLS)

	// Instantiate a new endpoint for SRTP from multiplexer
	srtpEndpoint := dataMux.NewEndpoint(func(b []byte) bool {
		// First byte looks like 10??????, representing RTP version 2.
		return b[0]&0xb0 == 0x80
	})

	// Configuration for DTLS handshake, namely certificate and private key
	config := &dtls.Config{Certificate: pc.certificate, PrivateKey: pc.privateKey}

	// Initiate a DTLS handshake as a client
	dtlsConn, err := dtls.Client(dtlsEndpoint, config)
	if err != nil {
		return err
	}

	// Create SRTP keys from DTLS handshake (see RFC5764 Section 4.2)
	keys, err := dtlsConn.ExportKeyingMaterial("EXTRACTOR-dtls_srtp", nil, 2*keyLen+2*saltLen)
	if err != nil {
		return err
	}
	keyReader := bytes.NewBuffer(keys)
	writeKey := keyReader.Next(keyLen)
	readKey := keyReader.Next(keyLen)
	writeSalt := keyReader.Next(saltLen)
	readSalt := keyReader.Next(saltLen)

	rtpSession := rtp.NewSession(rtp.SessionOptions{
		MuxConn:   srtpEndpoint, // rtcp-mux assumed
		ReadKey:   readKey,
		ReadSalt:  readSalt,
		WriteKey:  writeKey,
		WriteSalt: writeSalt,
	})

	videoStreamOpts := rtp.StreamOptions{
		Direction: "sendonly",
	}
	for _, m := range pc.localDescription.Media {
		if m.Type == "video" {
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &videoStreamOpts.LocalSSRC, &videoStreamOpts.LocalCNAME)
			break
		}
	}
	for _, m := range pc.remoteDescription.Media {
		if m.Type == "video" {
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &videoStreamOpts.RemoteSSRC, &videoStreamOpts.RemoteCNAME)
			break
		}
	}

	videoStream := rtpSession.AddStream(videoStreamOpts)
	go videoStream.SendVideo(pc.ctx.Done(), pc.DynamicType, pc.localVideo)

	//rtpSession, err := rtp.NewSecureSession(rtpEndpoint, readKey, readSalt, writeKey, writeSalt)
	//go streamH264(pc.ctx, pc.localVideoTrack, rtpSession.NewH264Stream(ssrc, cname))

	// Start goroutine for processing incoming SRTCP packets
	//go srtcpReaderRunloop(dataMux, readKey, readSalt)

	// Begin a new SRTP session
	//srtpSession, err := srtp.NewSession(srtpEndpoint, pc.DynamicType, writeKey, writeSalt)
	//if err != nil {
	//	return err
	//}

	// There are two termination conditions that we need to deal with here:
	// 1. Context cancellation. If Close() is called explicitly, or if the
	// parent context is canceled, we should terminate cleanly.
	// 2. Connection timeout. If the remote peer disconnects unexpectedly, the
	// read loop on the underlying net.UDPConn will time out. The associated
	// ice.DataStream will then be marked dead, which we check for here.
	select {
	case <-pc.ctx.Done():
		return nil
	case <-dataStream.Done():
		return dataStream.Err()
	}
}

// Close the peer connection
func (pc *PeerConnection) Close() {
	log.Info("Closing peer connection")

	// Cancel context to notify goroutines to exit.
	pc.cancel()
}
