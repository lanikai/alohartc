package main

//go:generate sh version.sh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/media/rtsp"
	"github.com/lanikai/alohartc/internal/signaling"
	"github.com/lanikai/alohartc/internal/v4l2"
)

var audioSource media.AudioSource
var videoSource media.VideoSource

func main() {
	flag.Parse()

	if flagHelp {
		help()
		os.Exit(0)
	}

	if flagVersion {
		version()
		os.Exit(0)
	}

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Open video source
	{
		err := fmt.Errorf("unsupported input: %s", flagInput)

		if strings.HasPrefix(flagInput, "rtsp://") {
			videoSource, err = rtsp.Open(flagInput)
		} else if strings.HasSuffix(flagInput, ".mp4") {
			videoSource, err = media.OpenMP4(flagInput)
		} else {
			var fi os.FileInfo
			if fi, err = os.Stat(flagInput); err == nil {
				// Assume device type files are Video4Linux2 devices
				if os.ModeDevice == fi.Mode()&os.ModeDevice {
					videoSource, err = v4l2.Open(flagInput, v4l2.Config{
						Width:                flagWidth,
						Height:               flagHeight,
						Bitrate:              1000 * flagBitrate,
						RepeatSequenceHeader: true,
					})
				} else {
					err = errors.New("Unrecognized device type")
				}
			}
		}

		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		if nil == videoSource {
			panic("logic error")
		}
		log.Printf("Local video: %dx%d %s\n", videoSource.Width(), videoSource.Height(), videoSource.Codec())
	}

	if closer, ok := videoSource.(io.Closer); ok {
		defer closer.Close()
	}

	if err := ice.Start(); err != nil {
		log.Fatal(err)
	}
	defer ice.Stop()

	signaling.Listen(doPeerSession)
}

func doPeerSession(ss *signaling.Session) {
	ctx, cancel := context.WithCancel(ss.Context)
	defer cancel()

	// Create peer connection with one video track
	pc := alohartc.Must(alohartc.NewPeerConnectionWithContext(
		ctx,
		alohartc.Config{
			LocalVideo: videoSource,
		}))
	defer pc.Close()

	// Register callback for ICE candidates produced by the local ICE agent.
	pc.OnIceCandidate = func(c *ice.Candidate) {
		if err := ss.SendLocalCandidate(c); err != nil {
			log.Fatal(err)
		}
	}

	// Wait for SDP offer from remote peer, then send our answer.
	select {
	case offer := <-ss.Offer:
		answer, err := pc.SetRemoteDescription(offer)
		if err != nil {
			log.Fatal(err)
		}

		if err := ss.SendAnswer(answer); err != nil {
			log.Fatal(err)
		}
	case <-ss.Done():
		log.Fatal(ss.Err())
	}

	// Pass remote candidates from the signaling server to the local ICE agent.
	go func() {
		for c := range ss.RemoteCandidates {
			pc.AddIceCandidate(&c)
		}
		pc.AddIceCandidate(nil)
	}()

	if err := pc.Stream(); err != nil {
		log.Println(err)
	}
}
