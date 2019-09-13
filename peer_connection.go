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
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/lanikai/alohartc/internal/dtls" // subtree merged pions/dtls
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/mux"
	"github.com/lanikai/alohartc/internal/rtp"
	"github.com/lanikai/alohartc/internal/sdp"
	"github.com/lanikai/alohartc/internal/srtp"
)

const (
	sdpUsername = "AlohaRTC"
	sdpUri      = "https://lanikailabs.com"

	keyLen  = 16
	saltLen = 14

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
	audioSource media.AudioSource
	videoSource media.VideoSource
	audioSink   media.AudioSink
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
		audioSource:      config.AudioSource,
		videoSource:      config.VideoSource,
		audioSink:        config.AudioSink,
		iceAgent:         ice.NewAgent(config.Interfaces),
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

// filterGroup returns bundle value for matching media types
func filterGroup(sess sdp.Session, allowedTypes map[string]bool) string {
	filtered := []string{"BUNDLE"}

	for _, m := range sess.Media {
		if allowed, ok := allowedTypes[m.Type]; ok && allowed {
			filtered = append(filtered, m.GetAttr("mid"))
		}
	}

	return strings.Join(filtered, " ")
}

// makeIceCredentials generates a random ice-ufrag and ice-pwd
func makeIceCredentials() (string, string, error) {
	// Require 24 and 128 bits of randomness for ufrag and pwd, respectively
	rnd := make([]byte, 3+16)
	if _, err := rand.Read(rnd); err != nil {
		return "", "", err
	}

	// Base64 encode ice-ufrag and ice-pwd
	ufrag := base64.StdEncoding.EncodeToString(rnd[0:3])
	pwd := base64.StdEncoding.EncodeToString(rnd[3:])

	return ufrag, pwd, nil
}

// Create SDP answer. Only needs SDP offer, no ICE candidates.
func (pc *PeerConnection) createAnswer() (sdp.Session, error) {
	// Generate ice-ufrag and ice-pwd
	ufrag, pwd, err := makeIceCredentials()
	if err != nil {
		return sdp.Session{}, err
	}

	isAudioAllowed := (nil != pc.audioSource) || (nil != pc.audioSink)
	isVideoAllowed := (nil != pc.videoSource)

	// Session-level block
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
		Uri:  sdpUri,
		Name: "-",
		Time: []sdp.Time{
			{nil, nil},
		},
		Attributes: []sdp.Attribute{
			{
				"group",
				filterGroup(
					pc.remoteDescription,
					map[string]bool{
						"audio": isAudioAllowed,
						"video": isVideoAllowed,
					},
				),
			},
		},
	}

	// Media-level blocks
	for _, remoteMedia := range pc.remoteDescription.Media {

		switch remoteMedia.Type {

		case "audio":
			port := 9

			// Must set port to zero when rejecting media block
			// See https://tools.ietf.org/html/rfc3264#section-6
			if !isAudioAllowed {
				port = 0
			}

			// Search rtpmap attributes for supported codecs
			acceptedPayloadTypes := make(map[int]interface{})
			for _, attr := range remoteMedia.Attributes {
				switch attr.Key {
				case "rtpmap":
					var pt int
					var codec string

					// Parse rtpmap line
					if _, err := fmt.Sscanf(
						attr.Value, "%3d %s", &pt, &codec,
					); err != nil {
						log.Warn("malformed rtpmap")
						break
					}

					// Accept or omit codec
					switch codec {
					// Opus 48KHz stereo
					case "opus/48000/2":
						// TODO Add --without-opus support
						acceptedPayloadTypes[pt] = &sdp.OpusFormatParameters{}

					// mu-law 8KHz mono
					case "PCMU/8000":
						// TODO Add --without-pcmu support
						acceptedPayloadTypes[pt] = &sdp.PCMUFormatParameters{}
					}
				}
			}

			// No accepted payload types? Omit audio media block entirely.
			if 0 == len(acceptedPayloadTypes) {
				port = 0
			}

			// String array for media answer containing accepted payload types
			acceptedFormats := []string{}
			for pt, _ := range acceptedPayloadTypes {
				acceptedFormats = append(acceptedFormats, strconv.Itoa(int(pt)))
			}

			// Attributes for media answer
			attrs := []sdp.Attribute{
				{"fingerprint", "sha-256 " + strings.ToUpper(pc.fingerprint)},
				{"ice-ufrag", ufrag},
				{"ice-pwd", pwd},
				{"ice-options", "trickle"},
				{"ice-options", "ice2"},
				{"mid", remoteMedia.GetAttr("mid")},
				{"rtcp", "9 IN IP4 0.0.0.0"},
				{"setup", "active"},
				{"rtcp-mux", ""},
				{"rtcp-rsize", ""},
			}
			if 0 != port {
				attrs = append(attrs, sdp.Attribute{
					sendrecv(pc.audioSource, pc.audioSink), "",
				})
			}
			for pt, fmtp := range acceptedPayloadTypes {
				switch fmtp.(type) {
				case *sdp.OpusFormatParameters:
					attrs = append(attrs, []sdp.Attribute{
						{"rtpmap", fmt.Sprintf("%d opus/48000/2", pt)},
						{"fmtp", fmt.Sprintf("%d minptime=10; useinbandfec=1", pt)},
						{"ptime", "20"},
					}...)
				case *sdp.PCMUFormatParameters:
					attrs = append(attrs, sdp.Attribute{
						"rtpmap",
						fmt.Sprintf("%d PCMU/8000", pt),
					})
				}
			}
			// TODO: Randomize SSRC (note: must be different from video SSRC)
			attrs = append(attrs, sdp.Attribute{
				"ssrc",
				"2541098698 cname:cYhx/N8U7h7+3GW5",
			})

			// Create media answer block
			media := sdp.Media{
				Type:   remoteMedia.Type,
				Port:   port,
				Proto:  "UDP/TLS/RTP/SAVPF",
				Format: acceptedFormats,
				Connection: &sdp.Connection{
					NetworkType: "IN",
					AddressType: "IP4",
					Address:     "0.0.0.0",
				},
				Attributes: attrs,
			}

			s.Media = append(s.Media, media)

		case "video":
			if nil == pc.videoSource {
				continue
			}

			// Select H.264 codec with packetization-mode=1 (only supported)
			supportedPayloadTypes := make(map[int]interface{})

			// search rtpmap attributes for supported codecs
			for _, attr := range remoteMedia.Attributes {
				switch attr.Key {
				case "rtpmap":
					var payloadType int
					var codec string

					// parse rtpmap line
					if _, err := fmt.Sscanf(
						attr.Value, "%3d %s", &payloadType, &codec,
					); err != nil {
						log.Warn("malformed rtpmap")
						break // switch
					}

					// only H.264 codec supported
					if "H264/90000" == codec {
						supportedPayloadTypes[payloadType] = &sdp.H264FormatParameters{}
					}
				}
			}

			// search fmtp attributes for supported format parameters
			for _, attr := range remoteMedia.Attributes {
				switch attr.Key {
				case "fmtp":
					var payloadType int
					var params string

					// parse fmtp line
					if _, err := fmt.Sscanf(
						attr.Value, "%3d %s", &payloadType, &params,
					); err != nil {
						log.Warn("malformed fmtp")
						break // switch
					}

					// only packetization-mode=1 supported
					if fmtp, ok := supportedPayloadTypes[payloadType]; ok {
						switch fmtp.(type) {
						case *sdp.H264FormatParameters:
							if err := fmtp.(*sdp.H264FormatParameters).Unmarshal(params); err != nil {
								log.Warn(err.Error())
							}
							if 1 != fmtp.(*sdp.H264FormatParameters).PacketizationMode {
								delete(supportedPayloadTypes, payloadType)
							}
						}
					}
				}
			}

			// choose first supported payload type
			var fmtp string
			for payloadType, formatParameters := range supportedPayloadTypes {
				switch formatParameters.(type) {
				case *sdp.H264FormatParameters:
					fmtp = formatParameters.(*sdp.H264FormatParameters).Marshal()
				}
				pc.DynamicType = uint8(payloadType)
				break
			}

			m := sdp.Media{
				Type:   remoteMedia.Type,
				Port:   9,
				Proto:  "UDP/TLS/RTP/SAVPF",
				Format: []string{strconv.Itoa(int(pc.DynamicType))},
				Connection: &sdp.Connection{
					NetworkType: "IN",
					AddressType: "IP4",
					Address:     "0.0.0.0",
				},
				Attributes: []sdp.Attribute{
					{"fingerprint", "sha-256 " + strings.ToUpper(pc.fingerprint)},
					{"ice-ufrag", ufrag},
					{"ice-pwd", pwd},
					{"ice-options", "trickle"},
					{"ice-options", "ice2"},
					{"mid", remoteMedia.GetAttr("mid")},
					{"rtcp", "9 IN IP4 0.0.0.0"},
					{"setup", "active"},
					{"sendonly", ""},
					{"rtcp-mux", ""},
					{"rtcp-rsize", ""},
					{"rtpmap", fmt.Sprintf("%d H264/90000", pc.DynamicType)},
					{"fmtp", fmt.Sprintf("%d %s", pc.DynamicType, fmtp)},
					// TODO: Randomize SSRC
					{"ssrc", "2541098696 cname:cYhx/N8U7h7+3GW3"},
				},
			}
			s.Media = append(s.Media, m)

		default:
			log.Warn("unsupported media type")
		}
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
	config := &dtls.Config{
		Certificate: pc.certificate,
		PrivateKey:  pc.privateKey,
	}

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

	audioStreamOpts := rtp.StreamOptions{
		Direction: sendrecv(pc.audioSource, pc.audioSink),
	}
	videoStreamOpts := rtp.StreamOptions{
		Direction: sendrecv(pc.videoSource, nil),
	}
	for _, m := range pc.localDescription.Media {
		switch m.Type {
		case "audio":
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &audioStreamOpts.LocalSSRC, &audioStreamOpts.LocalCNAME)
		case "video":
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &videoStreamOpts.LocalSSRC, &videoStreamOpts.LocalCNAME)
		}
	}
	for _, m := range pc.remoteDescription.Media {
		switch m.Type {
		case "audio":
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &audioStreamOpts.RemoteSSRC, &audioStreamOpts.RemoteCNAME)
		case "video":
			fmt.Sscanf(m.GetAttr("ssrc"), "%d cname:%s", &videoStreamOpts.RemoteSSRC, &videoStreamOpts.RemoteCNAME)
		}
	}

	// Send local video -> remote peer
	if nil != pc.videoSource {
		videoStream := rtpSession.AddStream(videoStreamOpts)
		go videoStream.SendVideo(pc.ctx.Done(), pc.DynamicType, pc.videoSource)
	}

	// Start goroutine for processing incoming SRTCP packets
	// TODO: Add back in
	//go srtcpReaderRunloop(dataMux, readKey, readSalt)

	// receive remote audio
	// TODO use proper codec based on payload type
	if nil != pc.audioSink {
		remoteAudioEndpoint := dataMux.NewEndpoint(mux.MatchSSRC(audioStreamOpts.RemoteSSRC))

		go func() {
			as := pc.audioSink

			// configure soundcard for 48 KHz stereo playback (Opus codec requirement)
			if err := as.Configure(48000, 2, media.S16LE); err != nil {
				log.Fatal(err)
			}

			// create audio buffer
			audioBuffer := make([]byte, 1280)

			// create srtp decryption context
			sess, err := srtp.NewSession(remoteAudioEndpoint, 0, readKey, readSalt)
			if err != nil {
				log.Fatal(err)
			}

			// instantiate decoder
			decoder, err := media.NewOpusDecoder(false)
			if err != nil {
				log.Fatal(err)
			}

			for {
				// read next audio packet (opus sends one frame per packet)
				n, err := sess.Read(audioBuffer)
				if err != nil {
					log.Println(err)
					if err == io.EOF {
						break
					}
					continue
				}

				// decode packet
				decoded, err := decoder.Decode(audioBuffer[:n-10])
				if err != nil {
					log.Println(err)
					continue
				}

				// write decoded packet to soundcard
				if n, err := as.Write(decoded); err != nil {
					log.Println(n, err)
				}
			}
		}()
	}

	// Goroutine for sending local audio track to remote peer
	if nil != pc.audioSource {
		localAudioEndpoint := dataMux.NewEndpoint(mux.MatchSSRC(audioStreamOpts.LocalSSRC))

		// create srtp decryption context
		sess, err := srtp.NewAudioSession(localAudioEndpoint, 111, writeKey, writeSalt)
		if err != nil {
			log.Fatal(err)
		}

		go sendAudioTrack(pc.ctx, sess, pc.audioSource)
	}

	// There are two termination conditions that we need to deal with here:
	// 1. Context cancellation. If Close() is called explicitly, or if the
	// parent context is canceled, we should terminate cleanly.
	// 2. Connection timeout. If the remote peer disconnects unexpectedly, the
	// read loop on the underlying net.UDPConn will time out. The associated
	// ice.DataStream will then be marked dead, which we check for here.
	select {
	case <-pc.ctx.Done():
		log.Println("context done")
		return nil
	case <-dataStream.Done():
		log.Println("datastream done")
		return dataStream.Err()
	}
}

// Close the peer connection
func (pc *PeerConnection) Close() {
	log.Info("Closing peer connection")

	// Cancel context to notify goroutines to exit.
	pc.cancel()
}

// sendAudioTrack transmits the local audio track to remote peer
// Terminates either on track read error or SRTP write error.
func sendAudioTrack(ctx context.Context, conn *srtp.AudioConn, as media.AudioSource) error {
	s := as.Subscribe(16)

	for {
		select {
		// Read audio (already encoded with selected codec)
		case p, ok := <-s:
			if !ok {
				return errors.New("audio source closed")
			}
			if err := conn.Send(p); err != nil {
				return err
			}
		// Abort on context termination
		case <-ctx.Done():
			as.Unsubscribe(s)

			return nil
		}
	}

	panic("logic error")
}

// sendrecv returns the direction attribute based on source/sink availability
func sendrecv(source media.MediaSource, sink media.MediaSink) string {
	switch {
	case nil == source && nil != sink:
		return "recvonly"
	case nil != source && nil == sink:
		return "sendonly"
	case nil != source && nil != sink:
		return "sendrecv"
	default:
		return ""
	}
}
