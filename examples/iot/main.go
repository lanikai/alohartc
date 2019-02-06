package main

import (
	"flag"
	"io"
	"log"
	"os"
	"strings"

	"github.com/lanikailabs/alohartc"
	"github.com/lanikailabs/alohartc/internal/signaling"
	"github.com/lanikailabs/alohartc/internal/v4l2"
)

// Flags
var (
	// Path to H.264 video source
	flagVideoSource string

	// Video parameters when using v4l2 source
	flagVideoBitrate int
	flagVideoWidth   uint
	flagVideoHeight  uint

	flagVideoHflip bool
	flagVideoVflip bool

	flagConfigPath string
)

func init() {
	flag.StringVar(&flagVideoSource, "i", "/dev/video0", "H.264 video source ('-' for stdin)")
	flag.IntVar(&flagVideoBitrate, "b", 2e6, "Bitrate for v4l2 video")
	flag.UintVar(&flagVideoWidth, "w", 1280, "Width for v4l2 video")
	flag.UintVar(&flagVideoHeight, "h", 720, "Height for v4l2 video")
	flag.BoolVar(&flagVideoHflip, "hflip", false, "Flip video horizontally")
	flag.BoolVar(&flagVideoVflip, "vflip", false, "Flip video vertically")
	flag.StringVar(&flagConfigPath, "c", "config.json", "Path to signal service config.json file")
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	cli, err := signaling.NewClient(flagConfigPath)
	if err != nil {
		log.Fatalf("%v\n", err)
	}

	// Connect to AWS IoT, listen for new sessions.
	if err := cli.Connect(); err != nil {
		log.Fatal(err)
	}

	session, err := cli.WaitForSession()
	if err != nil {
		log.Fatal(err)
	}

	doSession(session)
}

func doSession(session *signaling.Session) {
	pc := alohartc.NewPeerConnection()
	for {
		t, p := session.ReceiveMessage()
		switch t {
		case "status":
			log.Printf("Client status: %v\n", p)
			session.SendMessage("status", map[string]string{
				"status": "Connected",
			})
			// TODO: Detect disconnect
		case "offer":
			offer := p.(string)
			log.Printf("SDP offer: %s\n", offer)
			answer, err := pc.SetRemoteDescription(offer)
			if err != nil {
				log.Fatalf("%v", err)
			}

			session.SendMessage("answer", answer)

			// Send local ICE candidates through the signaling service, as they become available.
			lcand := make(chan string, 16)
			go func() {
				for cand := range lcand {
					log.Println("Local ICE", cand)
					session.SendMessage("iceCandidate", map[string]string{
						"candidate": cand,
						"sdpMid":    pc.SdpMid(),
					})
				}
				log.Println("End of local ICE candidates")
				session.SendMessage("iceCandidate", map[string]string{
					"candidate": "",
					"sdpMid":    pc.SdpMid(),
				})
			}()

			go func() {
				if err := pc.Connect(lcand); err != nil {
					log.Fatalf("%v", err)
				}
				defer pc.Close()

				streamVideo(pc)
			}()
		case "iceCandidate":
			cand := p.(string)
			if cand == "" {
				log.Println("End of remote ICE candidates")
			} else {
				log.Println("Remote ICE", cand)
			}
			pc.AddRemoteCandidate(cand)
		}
	}
}

func streamVideo(pc *alohartc.PeerConnection) {
	var source io.Reader
	wholeNALUs := false

	// Open the video source, either a v42l device, stdin, or a plain file.
	if strings.HasPrefix(flagVideoSource, "/dev/video") {
		v, err := v4l2.OpenH264(flagVideoSource, flagVideoWidth, flagVideoHeight)
		if err != nil {
			log.Fatal(err)
		}
		defer v.Close()

		if err := v.SetBitrate(flagVideoBitrate); err != nil {
			log.Fatal(err)
		}

		if flagVideoHflip {
			if err := v.FlipHorizontal(); err != nil {
				log.Fatal(err)
			}
		}

		if flagVideoVflip {
			if err := v.FlipVertical(); err != nil {
				log.Fatal(err)
			}
		}

		// Start video
		if err := v.Start(); err != nil {
			log.Fatal(err)
		}
		defer v.Stop()

		source = v
		wholeNALUs = true
	} else if flagVideoSource == "-" {
		source = os.Stdin
	} else {
		f, err := os.Open(flagVideoSource)
		if err != nil {
			log.Fatal(err)
		}
		source = f
	}

	if err := pc.StreamH264(source, wholeNALUs); err != nil {
		log.Fatal(err)
	}
}
