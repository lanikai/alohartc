package main

import (
	"fmt"
	"time"

	"github.com/lanikai/alohartc/internal/aes"
)

const payloadSize = 1280

var key = []byte("TopSecret128bits")

func main() {
	iv := make([]byte, aes.BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	start := time.Now()
	count := 0
	interval := 3 * time.Second
	payload := make([]byte, payloadSize)
	for time.Now().Sub(start) < interval {
		//block.CounterMode(iv).XORKeyStream(payload, payload)
		block.CounterModeEncrypt(iv, payload)
		count++
	}

	rate := float32(count*len(payload)) / float32(1024*1024) / float32(interval/time.Second)
	fmt.Printf("%d iterations of %d-byte AES-128-CTR in %v (%f MB/s)\n", count, payloadSize, interval, rate)
}
