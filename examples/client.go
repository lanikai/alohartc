package main

import (
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/thinkski/webrtc"
	"github.com/thinkski/webrtc/ice"
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

func doPeerConnection(ws *websocket.Conn, remoteDesc string, remoteCandidates <-chan string) {
	iceAgent := ice.NewAgent()

	pc := webrtc.NewPeerConnection()
	pc.SetRemoteDescription(remoteDesc)

	// Answer
	localDesc, _ := pc.CreateAnswer()
	if err := ws.WriteJSON(message{Type: "answer", Text: localDesc.String()}); err != nil {
		log.Fatal(err)
	}
	log.Println("sent answer")

	// Send local ICE candidates.
	localCandidates, err := iceAgent.GatherLocalCandidates()
	if err != nil {
		log.Fatal(err)
	}
	iceParams := map[string]string {"sdpMid": localDesc.GetMedia().GetAttr("mid")}
	for _, c := range localCandidates {
		log.Println("Local ICE", c.String())
		ws.WriteJSON(message{Type: "iceCandidate", Text: c.String(), Params: iceParams})
	}
	// Plus an empty candidate to indicate the end of the list.
	ws.WriteJSON(message{Type: "iceCandidate"})

	// Wait for remote ICE candidates.
	for {
		// TODO: use a reasonable timeout
		rc := <-remoteCandidates
		if rc == "" {
			log.Println("End of remote ICE candidates")
			break  // Empty string means there are no more candidates.
		}
		log.Println("Remote ICE", rc)
		iceAgent.AddRemoteCandidate(rc)
	}

	_, err = iceAgent.EstablishConnection(pc.Username(), pc.LocalPassword(), pc.RemotePassword())
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
