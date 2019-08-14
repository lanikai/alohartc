//////////////////////////////////////////////////////////////////////////////
//
// Broadcast byte slices from one writer to multiple subscribers.
//
// Each subscriber has its own channel (i.e. queue). When a writer
// broadcasts a byte slice, the byte slice is added to each subscriber's
// channel. Note that this is a shallow copy -- the data within the slice
// is not copied. When the byte slice is no longer referenced by any
// subscriber channel, it is freed by Go's garbage collector.
//
// Each subscriber may specify the maximum number of byte slices it
// wishes to buffer. Once this capacity is reached, the oldest byte slice
// is dropped for each new written byte slice.
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"sync"
)

type Subscriber interface {
	Subscribe(n int) <-chan []byte
	Unsubscribe(s <-chan []byte) error
}

// Broadcaster implements the io.WriteCloser and Subscriber interfaces
type Broadcaster struct {
	mutex       sync.RWMutex
	subscribers []chan []byte
}

// NewBroadcast instantiates a new one-to-many byte slice broadcaster
func NewBroadcaster() *Broadcaster {
	// Array of channels (one per subscriber), each channel containing byte slices
	return &Broadcaster{
		subscribers: []chan []byte{},
	}
}

// Close the broadcaster. All subscribers receive a channel closed message,
// their channels are drained, and the subscribers are deleted. Writes
// return an error.
func (b *Broadcaster) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Close each subscriber channel
	for _, subscriber := range b.subscribers {
		close(subscriber)
		for len(subscriber) > 0 {
			<-subscriber // Drain
		}
	}

	// Allow subscriber channels to be garbage collected
	b.subscribers = []chan []byte{}

	return nil
}

// Subscribe to broadcasts, buffering up to n byte slices for the subscriber
func (b *Broadcaster) Subscribe(n int) <-chan []byte {
	if n < 1 {
		panic("malformed buffer size")
	}

	// Create a new _buffered_ channel
	channel := make(chan []byte, n)
	b.mutex.Lock()
	b.subscribers = append(b.subscribers, channel)
	b.mutex.Unlock()
	return channel
}

// Unsubscribe from broadcaster by providing the read-only channel returned
// by Subscribe().
func (b *Broadcaster) Unsubscribe(s <-chan []byte) error {
	for i, subscriber := range b.subscribers {
		if s == subscriber {
			// Remove subscriber from slice (order not preserved)
			b.mutex.Lock()
			subs := b.subscribers
			close(subs[i])
			subs[len(subs)-1], subs[i] = subs[i], subs[len(subs)-1]
			b.subscribers = subs[:len(subs)-1]
			b.mutex.Unlock()
			return nil
		}
	}

	return errNotFound
}

// Write buffer to subscribers
func (b *Broadcaster) Write(p []byte) (n int, err error) {
	// Add slice to each subscriber
	for _, subscriber := range b.subscribers {
		select {
		case subscriber <- p:
			// Added slice reference to subscriber
		default:
			// Subscriber backlogged. Drop oldest byte slice, add newest.
			b.mutex.Lock()
			<-subscriber
			subscriber <- p
			b.mutex.Unlock()
		}
	}

	return len(p), nil
}
