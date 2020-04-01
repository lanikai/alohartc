package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/ice/mdns"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/media/rtsp"
	"github.com/lanikai/alohartc/internal/signaling"
	"github.com/lanikai/alohartc/internal/v4l2"
)

// Populated via -ldflags="-X ...". See Makefile.
var GitRevisionId string
var GitTag string

// Print version information
func version() {
	fmt.Println("🌈 Alohacam", GitRevisionId)
	fmt.Println("Copyright", time.Now().Year(), "Lanikai Labs LLC. All rights reserved.")
	fmt.Println("")
}

var audioSource media.AudioSource
var videoSource media.VideoSource

func main() {
	// Define and parse optional flags
	input := flag.String("i", "/dev/video0", "video input ('-' for stdin)")
	bitrate := flag.Int("bitrate", 1500e3, "set video bitrate")
	width := flag.Int("width", 1280, "set video width")
	height := flag.Int("height", 720, "set video height")
	flag.Parse()

	// Always print version information
	version()

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Open video source
	{
		err := fmt.Errorf("unsupported input: %s", *input)
		if strings.HasPrefix(*input, "/dev/video") {
			videoSource, err = v4l2.Open(*input, v4l2.Config{
				Width:   *width,
				Height:  *height,
				Bitrate: *bitrate,

				RepeatSequenceHeader: true,
			})
		} else if strings.HasPrefix(*input, "rtsp://") {
			videoSource, err = rtsp.Open(*input)
		} else if strings.HasSuffix(*input, ".mp4") {
			videoSource, err = media.OpenMP4(*input)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot open %s (%s)\n", *input, err)
			os.Exit(1)
		}

		log.Printf("Local video: %dx%d %s\n", videoSource.Width(), videoSource.Height(), videoSource.Codec())
	}

	if closer, ok := videoSource.(io.Closer); ok {
		defer closer.Close()
	}

	if err := mdns.Start(); err != nil {
		log.Fatal(err)
	}
	defer mdns.Stop()

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
