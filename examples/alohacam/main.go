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
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/media/rtsp"
	"github.com/lanikai/alohartc/internal/signaling"
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

//var source alohartc.MediaSource
var videoSource media.VideoSource

func main() {
	// Define and parse optional flags
	input := flag.String("i", "/dev/video0", "video input ('-' for stdin)")
	//bitrate := flag.Uint("bitrate", 1500e3, "set video bitrate")
	//width := flag.Uint("width", 1280, "set video width")
	//height := flag.Uint("height", 720, "set video height")
	//hflip := flag.Bool("hflip", false, "flip video horizontally")
	//vflip := flag.Bool("vflip", false, "flip video vertically")
	flag.Parse()

	// Always print version information
	version()

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Open media source
	{
		err := fmt.Errorf("unsupported input: %s", *input)
		if strings.HasPrefix(*input, "/dev/video") {
			//source, err = alohartc.NewV4L2MediaSource(
			//	*input,
			//	*width,
			//	*height,
			//	*bitrate,
			//	*hflip,
			//	*vflip,
			//)
		} else if strings.HasPrefix(*input, "rtsp://") {
			videoSource, err = rtsp.Open(*input)
		} else if strings.HasSuffix(*input, ".mp4") {
			videoSource, err = media.OpenMP4(*input)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot open %s (%s)\n", *input, err)
			os.Exit(1)
		}
	}

	if closer, ok := videoSource.(io.Closer); ok {
		defer closer.Close()
	}

	signaling.Listen(doPeerSession)
}

func doPeerSession(ss *signaling.Session) {
	ctx, cancel := context.WithCancel(ss.Context)
	defer cancel()

	// Get new track from media source. Close it when session ends.
	//track := source.GetTrack()
	//defer source.CloseTrack(track)

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
		fmt.Printf("Offer: %s\n", offer)
		answer, err := pc.SetRemoteDescription(offer)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Answer: %s\n", answer)

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
