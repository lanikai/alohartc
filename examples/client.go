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
}

func doPeerConnection(ws *websocket.Conn, remoteDesc string, remoteCandidates <-chan string) {
	ice := webrtc.NewIceAgent()

	pc := webrtc.NewPeerConnection()
	pc.SetRemoteDescription(remoteDesc)

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

	// Wait for remote ICE candidates.
	for {
		// TODO: use a reasonable timeout
		rc := <-remoteCandidates
		if rc == "" {
			log.Println("End of remote ICE candidates")
			break  // Empty string means there are no more candidates.
		}
		log.Println("Adding remote ICE candidate:", rc)
		ice.AddRemoteCandidate(rc)
	}

	ice.CheckConnectivity()
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

	remoteCandidates := make(chan string, 16)

	// Handle websocket messages
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
			go doPeerConnection(ws, msg.Text, remoteCandidates)
		case "iceCandidate":
			remoteCandidates <- msg.Text
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
