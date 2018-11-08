package main

import (
	"log"

	"github.com/thinkski/webrtc/internal/v4l2"
)

func main() {
	// Open
	dev, err := v4l2.OpenH264("/dev/video0", 1280, 720)
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	// Start
	if err := dev.Start(); err != nil {
		panic(err)
	}
	defer dev.Stop()

	// Read buffer after buffer
	p := make([]byte, 1000000)
	for {
		if n, err := dev.Read(p); err != nil {
			panic(err)
		} else {
			log.Println("Bytes read = ", n)
		}
	}
}
