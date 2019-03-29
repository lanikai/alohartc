package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/ice"
)

// Populated via -ldflags="-X ...". See Makefile.
var BuildDate string
var GitRevisionId string

// Flags
var (
	// HTTP port on which to listen
	flagPort int
)

func init() {
	flag.IntVar(&flagPort, "p", 8000, "HTTP port on which to listen")
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

// websocketHandler returns a parameterized websocket handler
func websocketHandler(ms alohartc.MediaSource) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Upgrade websocket connection
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		defer ws.Close()

		// Get new track from media source. Close it when session ends.
		track := ms.GetTrack()
		defer ms.CloseTrack(track)

		// Create peer connection with one video track
		pc := alohartc.Must(alohartc.NewPeerConnection(alohartc.Config{
			VideoTrack: track,
		}))
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

				if err := pc.Connect(); err != nil {
					log.Println(err)
					return
				}

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
	var source alohartc.MediaSource
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

	// Routes
	http.HandleFunc("/", indexHandler)
	http.Handle("/static/", http.StripPrefix("/static/", StaticServer()))
	http.HandleFunc("/ws", websocketHandler(source))

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
