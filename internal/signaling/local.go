// +build !release

package signaling

// See ./localdata/gen.go for "go generate" command used to bundle static files.

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/lanikai/alohartc/internal/ice"
	"github.com/lanikai/alohartc/internal/signaling/localdata"
)

var (
	// HTTP port on which to listen
	flagPort int
)

func init() {
	flag.IntVar(&flagPort, "p", 8000, "HTTP port on which to listen")

	RegisterListener(localWebsocketListener)
}

// Serve a static web page that uses a WebSocket for signaling. This is meant
// for development and debugging only.
func localWebsocketListener(handle SessionHandler) error {
	router := http.NewServeMux()
	router.Handle("/", http.FileServer(localdata.FS(false)))
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocketHandler(w, r, handle)
	})
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", flagPort),
		Handler: router,
	}

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

	fmt.Printf("Open http://%s/ in a browser\n", url)
	return server.ListenAndServe()
}

func websocketHandler(w http.ResponseWriter, r *http.Request, handle SessionHandler) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Upgrade websocket connection
	ws, err := new(websocket.Upgrader).Upgrade(w, r, nil)
	if err != nil {
		log.Warn("upgrade: %v", err)
		return
	}
	defer ws.Close()

	offerCh := make(chan string)
	rcandCh := make(chan ice.Candidate)
	session := &Session{
		Context:          ctx,
		Offer:            offerCh,
		RemoteCandidates: rcandCh,
		SendAnswer: func(sdp string) error {
			return ws.WriteJSON(map[string]string{
				"type": "answer",
				"sdp":  sdp,
			})
		},
		SendLocalCandidate: func(c *ice.Candidate) error {
			msg := map[string]string{"type": "iceCandidate"}
			if c != nil {
				msg["candidate"] = c.String()
				msg["sdpMid"] = c.Mid()
			}
			return ws.WriteJSON(msg)
		},
	}

	go handle(session)

	// Process incoming websocket messages. We expect JSON messages of the following form:
	//   { "type": "offer", "sdp": "..." }
	//   { "type": "iceCandidate", "candidate": "...", "sdpMid": "..." }
	for {
		msg := map[string]string{}
		if err := ws.ReadJSON(&msg); err != nil {
			log.Warn("Failed to read websocket message: %v", err)
			return
		}

		switch msg["type"] {
		case "offer":
			offerCh <- msg["sdp"]
		case "iceCandidate":
			if _, ok := msg["candidate"]; !ok {
				// An empty candidate indicates the end of ICE trickling.
				close(rcandCh)
				break
			}
			c, err := ice.ParseCandidate(msg["candidate"], msg["sdpMid"])
			if err != nil {
				log.Warn("Invalid ICE candidate '%s': %v", msg["candidate"], err)
			} else {
				rcandCh <- c
			}
		default:
			log.Warn("Unexpected websocket message: %v", msg)
		}
	}
}
