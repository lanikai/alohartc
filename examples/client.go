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
			if err := ws.WriteMessage(
				websocket.TextMessage,
				[]byte(sdp),
			); err != nil {
				log.Println("failed to send answer:", err)
				ws.Close()
				break
			}
			log.Println("sent answer")
		case "iceCandidate":
			log.Println("ice candidate")
			err := pc.AddIceCandidate(msg.Text)
			if err != nil {
				log.Println(err)
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
