package main

import (
	"log"

	"github.com/lanikai/alohartc/internal/v4l2"
)

func main() {
	// Open
	dev, err := v4l2.Open("/dev/video0", &v4l2.Config{
		Width:  1280,
		Height: 720,
		Format: v4l2.V4L2_PIX_FMT_H264,
	})
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
