package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/signaling"
)

// Populated via -ldflags="-X ...". See Makefile.
var BuildDate string
var GitRevisionId string

func version() {
	fmt.Println("🌈 Alohacam")

	if GitRevisionId != "" {
		fmt.Println("Git revision:\t", GitRevisionId)
	}

	if BuildDate != "" {
		fmt.Println("Build Date:\t", BuildDate)
	}

	fmt.Println("Copyright", time.Now().Year(), "Lanikai Labs. All rights reserved.")

	fmt.Println("")
}

var source alohartc.MediaSource

func main() {
	// Define and parse optional flags
	input := flag.String("i", "/dev/video0", "video input ('-' for stdin)")
	bitrate := flag.Uint("bitrate", 2e6, "set video bitrate")
	width := flag.Uint("width", 1280, "set video width")
	height := flag.Uint("height", 720, "set video height")
	hflip := flag.Bool("hflip", false, "flip video horizontally")
	vflip := flag.Bool("vflip", false, "flip video vertically")
	flag.Parse()

	// Always print version information
	version()

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Open media source
	{
		var err error
		if strings.HasPrefix(*input, "/dev/video") {
			source, err = alohartc.NewV4L2MediaSource(
				*input,
				*width,
				*height,
				*bitrate,
				*hflip,
				*vflip,
			)
		} else {
			source, err = alohartc.NewFileMediaSource(*input)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot open %s (%s)\n", *input, err)
			os.Exit(1)
		}
	}
	defer source.Close()

	signaling.Listen(doPeerSession)
}

func doPeerSession(ss *signaling.Session) {
	// Get new track from media source. Close it when session ends.
	track := source.GetTrack()
	defer source.CloseTrack(track)

	// Create peer connection with one video track
	pc := alohartc.Must(alohartc.NewPeerConnectionWithContext(
		ss.Context,
		alohartc.Config{
			VideoTrack: track,
		}))
	defer pc.Close()

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

	// Pass remote ICE candidates along to the PeerConnection.
	go func() {
		for c := range ss.RemoteCandidates {
			log.Println("Remote ICE", c)
			pc.AddIceCandidate(c)
		}
	}()

	// Send local ICE candidates to the remote peer.
	go func() {
		for c := range pc.LocalICECandidates() {
			log.Println("Local ICE", c)
			ss.SendLocalCandidate(c)
		}
	}()

	// Block until we're connected.
	if err := pc.Connect(); err != nil {
		log.Println(err)
	}
}
