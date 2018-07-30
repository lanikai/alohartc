package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/thinkski/webrtc"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// indexHandler serves index.html
func indexHandler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("web/templates/index.html"))
	t.Execute(w, nil)
}

type message struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
	pc := webrtc.NewPeerConnection()

	ice := webrtc.NewIceAgent()

	// Handle websocket messages
	for {
		// Read websocket message
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		// Unmarshal websocket message
		msg := message{}
		if err := json.Unmarshal(p, &msg); err != nil {
			log.Println(err)
		}

		// Parse message type
		switch msg.Type {
		case "offer":
			log.Println("offer")
			pc.SetRemoteDescription(msg.Text)

			// Answer
			sdp, _ := pc.CreateAnswer()
			if err := ws.WriteJSON(message{"answer", sdp}); err != nil {
				log.Fatal(err)
			}
			log.Println("sent answer")

			// Send ICE candidates
			localCandidates, err := ice.GatherCandidates()
			if err != nil {
				log.Fatal(err)
			}
			for _, c := range localCandidates {
				ws.WriteJSON(message{"iceCandidate", c.String()})
			}

			// Plus an empty candidate to indicate the end of the list.
			ws.WriteJSON(message{"iceCandidate", ""})

		case "iceCandidate":
			//err := pc.AddIceCandidate(msg.Text)
			if msg.Text != "" {
				err := ice.AddRemoteCandidate(msg.Text)
				if err != nil {
					log.Println(err)
				}
			} else {
				// An empty candidate means the remote side is ready for connectivity checks.
			}
		}
	}
}

// main function
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/ws", websocketHandler)

	// Static file handler
	http.Handle("/static/", http.StripPrefix(
		"/static/", http.FileServer(http.Dir("web/static")),
	))

	// Listen on port
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}
