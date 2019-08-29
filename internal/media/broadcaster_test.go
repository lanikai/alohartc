package media

import (
	"bytes"
	"sync"
	"testing"
)

func TestSubscribeAndWrite(t *testing.T) {

	b := NewBroadcaster()

	var wg sync.WaitGroup

	// Hundred subscribers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int, b *Broadcaster) {
			s := b.Subscribe(1)

			select {
			case p, ok := <-s:
				// Test that each one receives packet
				if !bytes.Equal(p, []byte{0xc0, 0xff, 0xee}) {
					t.Fail()
				}

				if !ok {
					t.Fail()
				}

				wg.Done()
			}
		}(i, b)
	}

	// Write packet repeatedly
	go func(b *Broadcaster) {
		packet := []byte{0xc0, 0xff, 0xee}
		for {
			b.Write(packet)
		}
	}(b)

	// Wait for all subscribers to receive packet
	wg.Wait()
}

func TestUnsubscribe(t *testing.T) {
	b := NewBroadcaster()

	ch := b.Subscribe(10)

	if err := b.Unsubscribe(ch); err != nil {
		t.Fail()
	}
}
