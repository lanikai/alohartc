package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"

	"github.com/lanikai/alohartc"
	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/v4l2"
)

// Flags
var (
	// HTTP port on which to listen
	flagPort int

	// Path to raw H.264 video source
	flagVideoSource string

	log = logging.DefaultLogger.WithTag("main")
)

func init() {
	flag.IntVar(&flagPort, "p", 8000, "HTTP port on which to listen")
	flag.StringVar(&flagVideoSource, "i", "v4l2:/dev/video0", "Video source spec")
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
			answer, err := pc.SetRemoteDescription(msg.Text)
			if err != nil {
				log.Println(err)
				return
			}
			ws.WriteJSON(message{Type: "answer", Text: answer})
			go sendIceCandidates(ws, lcand, pc.SdpMid())

			go func() {
				if err := pc.Connect(lcand); err != nil {
					log.Println(err)
					return
				}
				streamVideo(pc)
			}()

		case "iceCandidate":
			if msg.Text == "" {
				log.Println("End of remote ICE candidates")
			} else {
				log.Println("Remote ICE", msg.Text)
			}
			pc.AddRemoteCandidate(msg.Text)
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

func streamVideo(pc *alohartc.PeerConnection) {
	// Open the video source.
	src, err := media.OpenSource(flagVideoSource)
	if err != nil {
		log.Fatal(err)
	}

	// Open the video source.
	src, err := media.OpenSource(flagVideoSource)
	if err != nil {
		log.Fatal(err)
	}

	switch v := src.(type) {
	case media.H264Source:
		err = pc.StreamH264(v)
	default:
		log.Fatalf("Video source is not H.264: %s", flagVideoSource)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.Parse()

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
