package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/media/rtsp"
	"github.com/lanikai/alohartc/internal/signaling"
	"github.com/lanikai/alohartc/internal/v4l2"

	"github.com/pborman/getopt/v2"
)

// Populated via -ldflags="-X ...". See Makefile.
var GitRevisionId string
var GitTag string

// help displays usage information and exits successfully (GNU convention)
func help() {
	fmt.Println("alohartcd [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("      --audio-source=<str>    Audio capture device. Only ALSA devices supported.")
	fmt.Println("                                Set to \"none\" to omit from offer.")
	fmt.Println("                                (default: \"default\")")
	fmt.Println("      --audio-sink=<str>      Audio playback device. Only ALSA devices")
	fmt.Println("                                supported. Set to \"none\" to omit from offer.")
	fmt.Println("                                (default: \"default\")")
	fmt.Println("  -b, --bitrate=<int>         Local video bitrate, in bits per second")
	fmt.Println("                                (default: 1500000)")
	fmt.Println("  -g, --geometry=<int>x<int>  Local video frame size, in pixels")
	fmt.Println("                                (default: 1280x720)")
	fmt.Println("  -h, --help                  Display this message and exit successfully")
	fmt.Println("      --without-opus          Omit support for Opus codec")
	fmt.Println("      --without-pcm           Omit support for PCM μ-law codec")
	fmt.Println("      --video-source=<str>    Video capture device. Supported devices include")
	fmt.Println("                                Video4Linux2, RTSP, and MP4 files. Video source")
	fmt.Println("                                must produce H.264 encoded byte-stream. Set to")
	fmt.Println("                                \"none\" to omit from offer (default: /dev/video0)")
	fmt.Println("  -i, --interfaces=<std>      Comma-separated list of interfaces to use")
	fmt.Println("  -v, --version               Display version and exit successfully")
	fmt.Println("")
	fmt.Println("Please report bugs to <aloha@lanikailabs.com>. Mahalo!")
}

// version displays information and exits successfully (GNU convention)
func version() {
	fmt.Println("alohartcd", GitRevisionId)
	fmt.Println("Copyright 2019 Lanikai Labs LLC. All rights reserved.")
	fmt.Println("Visit https://lanikailabs.com for more information")
}

var audioSink media.AudioSink
var audioSource media.AudioSource
var videoSource media.VideoSource
var interfaces map[string]bool

// Command line flags
var (
	audioSinkFlag   = getopt.StringLong("audio-sink", 1000, "default", "")
	audioSourceFlag = getopt.StringLong("audio-source", 1001, "default", "")
	inputFlag       = getopt.StringLong("input", 'i', "/dev/video0", "")
	interfacesFlag  = getopt.StringLong("interfaces", 1003, "all", "")
	bitrateFlag     = getopt.IntLong("bitrate", 'b', 1500e3, "")
	geometryFlag    = getopt.StringLong("geometry", 'g', "1280x720", "")
	helpFlag        = getopt.BoolLong("help", 'h', ".")
	versionFlag     = getopt.BoolLong("version", 'v', "")
	videoSourceFlag = getopt.StringLong("video-source", 1002, "/dev/video0", "")
)

func main() {
	// Parse command line arguments
	getopt.Parse()

	// Check for help flag
	if *helpFlag {
		help()
		os.Exit(0)
	}

	// Check for version flag
	if *versionFlag {
		version()
		os.Exit(0)
	}

	// Parse input video frame geometry
	var width, height int
	if n, err := fmt.Sscanf(*geometryFlag, "%dx%d", &width, &height); n != 2 || err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Parse interfaces string
	interfaces = make(map[string]bool)
	for _, name := range strings.Split(*interfacesFlag, ",") {
		interfaces[name] = true
	}

	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Open audio sink
	if *audioSinkFlag != "none" {
		as, err := media.NewALSAAudioSink(*audioSinkFlag)
		if nil != err {
			log.Fatal(err)
		}
		defer as.Close()

		audioSink = as
	}

	// Open audio source
	if *audioSourceFlag != "none" {
		as, err := media.NewALSAAudioSource(*audioSourceFlag)
		if nil != err {
			log.Fatal(err)
		}
		defer as.Close()

		// TODO Configure later based on codec
		as.Configure(48000, 2, media.S16LE)

		audioSource = as
	}

	// Open video source
	if *videoSourceFlag != "none" {
		var err error

		// Video4Linux2
		if strings.HasPrefix(*videoSourceFlag, "/dev/video") {
			videoSource, err = v4l2.Open(*videoSourceFlag, v4l2.Config{
				Width:   width,
				Height:  height,
				Bitrate: *bitrateFlag,

				RepeatSequenceHeader: true,
			})

		} else if strings.HasPrefix(*videoSourceFlag, "rtsp://") {
			videoSource, err = rtsp.Open(*videoSourceFlag)

		} else if strings.HasSuffix(*videoSourceFlag, ".mp4") {
			videoSource, err = media.OpenMP4(*videoSourceFlag)
		}

		// Check for error
		if nil != err {
			log.Fatal(err)
		}

		// Defer close of video source
		if closer, ok := videoSource.(io.Closer); ok {
			defer closer.Close()
		}
	}

	signaling.Listen(doPeerSession)
}

func doPeerSession(ss *signaling.Session) {
	ctx, cancel := context.WithCancel(ss.Context)
	defer cancel()

	// Create peer connection with one video track
	pc := alohartc.Must(alohartc.NewPeerConnectionWithContext(
		ctx,
		alohartc.Config{
			AudioSink:   audioSink,
			AudioSource: audioSource,
			VideoSource: videoSource,
			Interfaces:  interfaces,
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
