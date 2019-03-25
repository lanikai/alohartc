// +build !oahu

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

	NewClient = newLocalWebSignaler
}

// A signaling.Client that also acts as the signaling server, running a local
// webserver that the browser connects to directly. Signaling is then performed
// over a websocket.
type localWebSignaler struct {
	handler SessionHandler
	server  *http.Server
}

func newLocalWebSignaler(handler SessionHandler) (Client, error) {
	router := http.NewServeMux()
	s := &localWebSignaler{
		handler: handler,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", flagPort),
			Handler: router,
		},
	}
	router.Handle("/", http.FileServer(localdata.FS(false)))
	router.HandleFunc("/ws", s.handleWebsocket)

	return s, nil
}

func (s *localWebSignaler) Listen() error {
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
	return s.server.ListenAndServe()
}

func (s *localWebSignaler) Shutdown() error {
	return s.server.Shutdown(context.Background())
}

func (s *localWebSignaler) handleWebsocket(w http.ResponseWriter, r *http.Request) {
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
		SendLocalCandidate: func(c ice.Candidate) error {
			return ws.WriteJSON(map[string]string{
				"type":      "iceCandidate",
				"candidate": c.String(),
				"sdpMid":    c.Mid(),
			})
		},
	}

	go s.handler(session)

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
