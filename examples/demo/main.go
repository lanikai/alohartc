package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/thinkski/webrtc"
	"github.com/thinkski/webrtc/internal/v4l2"
)

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
)

func init() {
	flag.IntVar(&flagPort, "p", 8000, "HTTP port on which to listen")
	flag.StringVar(&flagVideoSource, "i", "/dev/video0", "H.264 video source ('-' for stdin)")
	flag.IntVar(&flagVideoBitrate, "b", 2e6, "Bitrate for v4l2 video")
	flag.UintVar(&flagVideoWidth, "w", 1280, "Width for v4l2 video")
	flag.UintVar(&flagVideoHeight, "h", 720, "Height for v4l2 video")
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

	// Remote ICE candidates, sent to us over the Websocket connection.
	rcand := make(chan string, 16)
	// Local ICE candidates, produced by the local ICE agent.
	lcand := make(chan string, 16)

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
			pc := webrtc.NewPeerConnection()
			pc.SetRemoteDescription(msg.Text)
			ws.WriteJSON(message{Type: "answer", Text: pc.CreateAnswer()})
			go sendIceCandidates(ws, lcand, pc.SdpMid())
			go func() {
				if err := pc.Connect(lcand, rcand); err != nil {
					log.Fatal(err)
				}
				defer pc.Close()

				streamVideo(pc)
			}()
		case "iceCandidate":
			if msg.Text == "" {
				log.Println("End of remote ICE candidates")
				close(rcand)
			} else {
				log.Println("Remote ICE", msg.Text)
				rcand <- msg.Text
			}
		}
	}
}

// Relay local ICE candidates to the remote ICE agent as soon as they become available.
func sendIceCandidates(ws *websocket.Conn, lcand <-chan string, sdpMid string) {
	iceParams := map[string]string{"sdpMid": sdpMid}
	for c := range lcand {
		log.Println("Local ICE", c)
		ws.WriteJSON(message{Type: "iceCandidate", Text: c, Params: iceParams})
	}
	log.Println("End of local ICE candidates")
	// Plus an empty candidate to indicate the end of the list.
	ws.WriteJSON(message{Type: "iceCandidate"})
}

func streamVideo(pc *webrtc.PeerConnection) {
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

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Routes
	http.HandleFunc("/", indexHandler)
	http.Handle("/static/", http.StripPrefix("/static/", StaticServer()))
	http.HandleFunc("/ws", websocketHandler)

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	// Listen on port
	fmt.Printf("Demo is running. Open http://%s:%d in a browser.\n", hostname, flagPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil); err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}
