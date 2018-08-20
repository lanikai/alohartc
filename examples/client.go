package main

import (
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

		log.Printf("websocket message received: type = %s", msg.Type)
		switch msg.Type {
		case "offer":
			pc := webrtc.NewPeerConnection()
			pc.SetRemoteDescription(msg.Text)
			ws.WriteJSON(message{Type: "answer", Text: pc.CreateAnswer()})
			log.Println("sent answer")
			go sendIceCandidates(ws, lcand, pc.SdpMid())
			go pc.Connect(lcand, rcand)
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
	iceParams := map[string]string {"sdpMid": sdpMid}
	for c := range lcand {
		log.Println("Local ICE", c)
		ws.WriteJSON(message{Type: "iceCandidate", Text: c, Params: iceParams})
	}
	// Plus an empty candidate to indicate the end of the list.
	ws.WriteJSON(message{Type: "iceCandidate"})
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
