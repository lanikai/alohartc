package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/thinkski/webrtc/dtls"
)

func main() {
	var host string
	var port uint

	flag.StringVar(&host, "host", "localhost", "destination host")
	flag.UintVar(&port, "port", 4433, "destination port")

	flag.Parse()

	addr := fmt.Sprintf("%s:%d", host, port)
	log.Println(addr)
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Println("ahoy")
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Send DTLS client hello
	if _, err := dtls.DialWithConnection(conn); err != nil {
		log.Fatal(err)
	}
}
