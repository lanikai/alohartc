package main

import (
	"log"
	"net"
	"time"

	"./webrtc"
)

func main() {
	raddr, err := net.ResolveUDPAddr("udp", "localhost:2018")
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	srtp := webrtc.SRTP{
		PayloadType: 96,
		SSRC: 0x20180709,
		Data: []byte{ 'A', 'H', 'O', 'Y' },
	}

	for {
		b, err := srtp.MarshalBinary()
		if err != nil {
			log.Fatal(err)
		}
		conn.Write(b)

		time.Sleep(time.Second)
	}
}
