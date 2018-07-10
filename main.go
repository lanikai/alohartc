package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Command-line flags
var flagPort uint

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Initialization, run before main()
func init() {
	flag.UintVar(&flagPort, "port", 8000, "Listen on this port")
}

// indexHandler serves index.html
func indexHandler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("templates/index.html"))
	t.Execute(w, nil)
}

type message struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Date string `json:"date"`
}

type SDP struct {
}

func (sdp *SDP) UnmarshalBinary(b []byte) {
	buf := bytes.NewReader(b)

	scanner := bufio.NewScanner(buf)

	for scanner.Scan() {
		s := strings.SplitN(scanner.Text(), "=", 2)
		c, val := s[0], s[1]
		log.Println(c, val)
	}
}

// websocketHandler handles websocket connections
func websocketHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	defer ws.Close()

	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		msg := message{}
		if err := json.Unmarshal(p, &msg); err != nil {
			log.Println(err)
		}

		// Parse message type
		var sdp SDP
		switch msg.Type {
		case "onIceCandidate":
			log.Println(msg.Text)
		case "setLocalDescription":
			sdp.UnmarshalBinary([]byte(msg.Text))
		default:
		}
	}
}

// main function
func main() {
	flag.Parse()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/ws", websocketHandler)

	// Static file handler
	http.Handle("/static/", http.StripPrefix(
		"/static/", http.FileServer(http.Dir("static")),
	))

	// Listen on port
	addr := fmt.Sprintf(":%d", flagPort)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServer: ", err)
	}
}
