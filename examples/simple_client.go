package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

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

	//	// Load client certificate from file
	//	cert, err := LoadX509KeyPair("server.pem", "server-private.pem")
	//	if err != nil {
	//		log.Fatal(err)
	//	}

	// WebRTC certificate chain and host name are not set by browser(s)
	//	config.Certificates = append(config.Certificates, cert)
	config := &dtls.Config{
		ClientSessionCache: dtls.NewLRUClientSessionCache(1),
		InsecureSkipVerify: true,
		KeyLogWriter:       os.Stdout,
		MinVersion:         dtls.VersionDTLS12,
	}

	// Send DTLS client hello
	if _, err := dtls.DialWithConnection(conn, config); err != nil {
		log.Fatal(err)
	}
}
