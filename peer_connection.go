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

	"github.com/hajimehoshi/oto" // sound card interface
	"github.com/pd0mz/go-g711"   // g.711 decoding
)

const (
	sdpUsername = "AlohaRTC"
	sdpUri      = "https://lanikailabs.com"

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

// filterGroup returns bundle value for matching media types
func filterGroup(sess sdp.Session, allowedTypes map[string]bool) string {
	filtered := []string{"BUNDLE"}

	for _, m := range sess.Media {
		log.Println("type", m.Type)
		if _, ok := allowedTypes[m.Type]; ok {
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
			{"group", filterGroup(pc.remoteDescription, map[string]bool{"audio": true, "video": true})},
		},
	}

	for _, remoteMedia := range pc.remoteDescription.Media {

		switch remoteMedia.Type {

		case "audio":
			// Select PCMU codec (only supported)
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

					// only G.711 u-law (i.e. PCMU) codec supported
					if "PCMU/8000" == codec {
						supportedPayloadTypes[payloadType] = &sdp.PCMUFormatParameters{}
					}
				}
			}

			// choose first supported payload type
			var payloadType int
			for payloadType, _ = range supportedPayloadTypes {
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
					{"recvonly", ""},
					{"rtcp-mux", ""},
					{"rtcp-rsize", ""},
					{"rtpmap", fmt.Sprintf("%d PCMU/8000", payloadType)},
					{"msid", "SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c"},
					{"ssrc", "3841098696 cname:cYhx/N8U7h7+3GW3"},
				},
			}
			s.Media = append(s.Media, m)

		case "video":
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
							log.Info("H264FormatParameters")
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

	mid := offer.Media[1].GetAttr("mid")
	remoteUfrag := offer.Media[1].GetAttr("ice-ufrag")
	localUfrag := answer.Media[1].GetAttr("ice-ufrag")
	username := remoteUfrag + ":" + localUfrag
	localPassword := answer.Media[1].GetAttr("ice-pwd")
	remotePassword := offer.Media[1].GetAttr("ice-pwd")
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

	// Get remote audio SSRC
	remoteAudioSSRC := uint32(0)
	for _, media := range pc.remoteDescription.Media {
		if "audio" == media.Type {
			if ssrc, err := strconv.Atoi(strings.Fields(media.GetAttr("ssrc"))[0]); err != nil {
				return err
			} else {
				remoteAudioSSRC = uint32(ssrc)
			}
		}
	}

	// Instantiate a new endpoint for SRTP from multiplexer
	srtpEndpoint := dataMux.NewEndpoint(func(b []byte) bool {
		// First byte looks like 10??????, representing RTP version 2.
		return b[0]&0xb0 == 0x80
	})
	remoteAudioEndpoint := dataMux.NewEndpoint(mux.MatchSSRC(remoteAudioSSRC))

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

	go func() {
		// create audio context
		audioCtx, err := oto.NewContext(8000, 1, 2, 4096)
		if err != nil {
			log.Fatal(err)
		}
		defer audioCtx.Close()

		// create new player for context (??)
		player := audioCtx.NewPlayer()
		defer player.Close()

		// create file for writing audio
		//file, err := os.Create("remoteAudio.raw")
		//if err != nil {
		//	log.Fatal(err)
		//}

		// create audio buffer
		audioBuffer := make([]byte, 1280)

		// create srtp decryption context
		sess, err := srtp.NewSession(remoteAudioEndpoint, 0, readKey, readSalt)
		if err != nil {
			log.Fatal(err)
		}

		for {
			// read next audio packet
			n, err := sess.Read(audioBuffer)
			if err != nil {
				log.Fatal(err)
			}

			decoded := g711.MLawDecode([]uint8(audioBuffer[:n-10]))
			decodedBytes := make([]byte, (2 * len(decoded)))

			for i, val := range decoded {
				decodedBytes[2*i] = byte(val)
				decodedBytes[2*i+1] = byte(val >> 8)
			}

			buffer := bytes.NewBuffer(decodedBytes)

			// write audio packet to sound card
			io.Copy(player, buffer)

			// write audio packet to file
			//file.Write(audioBuffer[:n-10])
		}
	}()

	// Start a goroutine for sending each video track to connected peer.
	//if pc.localVideoTrack != nil {
	//	go sendVideoTrack(srtpSession, pc.localVideoTrack)
	//}

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

// sendVideoTrack transmits the local video track to the remote peer.
// Terminates either on track read error or SRTP write error.
func sendVideoTrack(conn *srtp.Conn, track Track) error {
	switch track.PayloadType() {
	case "H264/90000":
		var stap []byte
		buf := make([]byte, 128*1024)
		gotParameterSet := false

		for {
			// Read next NAL unit from H.264 video track
			n, err := track.Read(buf)
			if err != nil {
				return err
			}
			nalu := buf[:n]

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
				stap = append(stap, nalu...)

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
					if err := conn.Stap(stap); err != nil {
						return err
					}
					stap = nil
				}

				// Send NALU
				if err := conn.Send(nalu); err != nil {
					return err
				}
			}
		}

	default:
		panic("unsupported video track")
	}

	panic("should never get here")
}
