package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/v4l2"
)

// Populated via -ldflags="-X ...". See Makefile.
var BuildDate string
var GitRevisionId string

// Flags
var (

	// HTTP port on which to listen
	flagPort int

	// Path to raw H.264 video source
	flagVideoSource string

	// Video parameters when using v4l2 source
	flagVideoBitrate int
	flagVideoWidth   uint
	flagVideoHeight  uint

	flagVideoHflip bool
	flagVideoVflip bool
)

func init() {
	flag.IntVar(&flagPort, "p", 8000, "HTTP port on which to listen")
	flag.StringVar(&flagVideoSource, "i", "/dev/video0", "H.264 video source ('-' for stdin)")
	flag.IntVar(&flagVideoBitrate, "b", 2e6, "Bitrate for v4l2 video")
	flag.UintVar(&flagVideoWidth, "w", 1280, "Width for v4l2 video")
	flag.UintVar(&flagVideoHeight, "h", 720, "Height for v4l2 video")
	flag.BoolVar(&flagVideoHflip, "hflip", false, "Flip video horizontally")
	flag.BoolVar(&flagVideoVflip, "vflip", false, "Flip video vertically")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type message struct {
	Type   string            `json:"type"`
	Text   string            `json:"text"`
	Params map[string]string `json:"params,omitempty"`
}

// websocketHandler handles websocket connections
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade websocket connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()

	// Create peer connection
	pc := alohartc.NewPeerConnection(context.Background())
	defer pc.Close()

	// Handle incoming websocket messages
	for {
		// Read JSON message
		msg := message{}
		if err := ws.ReadJSON(&msg); err != nil {
			log.Println(err)
			return
		}

		switch msg.Type {
		case "offer":
			answer, err := pc.SetRemoteDescription(msg.Text)
			if err != nil {
				log.Println(err)
				return
			}
			ws.WriteJSON(message{Type: "answer", Text: answer})
			go sendIceCandidates(ws, pc.LocalICECandidates())

			go func() {
				if err := pc.Connect(); err != nil {
					log.Println(err)
					return
				}
				streamVideo(pc)
			}()

		case "iceCandidate":
			if msg.Text == "" {
				log.Println("End of remote ICE candidates")
				pc.AddIceCandidate("", "")
			} else {
				log.Println("Remote ICE", msg.Text)
				pc.AddIceCandidate(msg.Text, msg.Params["sdpMid"])
			}
		}
	}
}

// Relay local ICE candidates to the remote ICE agent as soon as they become available.
func sendIceCandidates(ws *websocket.Conn, lcand <-chan ice.Candidate) {
	for c := range lcand {
		log.Println("Local ICE", c)
		ws.WriteJSON(message{
			Type:   "iceCandidate",
			Text:   c.String(),
			Params: map[string]string{"sdpMid": c.Mid()},
		})
	}
	log.Println("End of local ICE candidates")
	// Plus an empty candidate to indicate the end of the list.
	ws.WriteJSON(message{Type: "iceCandidate"})
}

func streamVideo(pc *alohartc.PeerConnection) {
	var source io.Reader
	wholeNALUs := false

	// Open the video source, either a v42l device, stdin, or a plain file.
	if strings.HasPrefix(flagVideoSource, "/dev/video") {
		v, err := v4l2.Open(flagVideoSource, &v4l2.Config{
			Width:                flagVideoWidth,
			Height:               flagVideoHeight,
			Format:               v4l2.V4L2_PIX_FMT_H264,
			RepeatSequenceHeader: true,
		})
		if err != nil {
			log.Println(err)
			return
		}
		defer v.Close()

		if err := v.SetBitrate(flagVideoBitrate); err != nil {
			log.Println(err)
			return
		}

		if flagVideoHflip {
			if err := v.FlipHorizontal(); err != nil {
				log.Println(err)
				return
			}
		}

		if flagVideoVflip {
			if err := v.FlipVertical(); err != nil {
				log.Println(err)
				return
			}
		}

		// Start video
		if err := v.Start(); err != nil {
			log.Println(err)
			return
		}
		defer v.Stop()

		source = v
		wholeNALUs = true
	} else if flagVideoSource == "-" {
		source = os.Stdin
	} else {
		f, err := os.Open(flagVideoSource)
		if err != nil {
			log.Println(err)
			return
		}
		source = f
	}

	if err := pc.StreamH264(source, wholeNALUs); err != nil {
		log.Println(err)
	}
}

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

func main() {
	version()

	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Routes
	http.HandleFunc("/", indexHandler)
	http.Handle("/static/", http.StripPrefix("/static/", StaticServer()))
	http.HandleFunc("/ws", websocketHandler)

	// Get hostname
	url, err := os.Hostname()
	if err != nil {
		url = "localhost"
	} else if strings.IndexAny(url, ".") == -1 {
		url += ".local"
	}
	if flagPort != 80 {
		url += fmt.Sprintf(":%d", flagPort)
	}

	// Listen on port
	fmt.Printf("Open http://%s/ in a browser\n", url)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil); err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}
