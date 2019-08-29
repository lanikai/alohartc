package media

import (
	"sync"
)

// Flow is a concrete implementation of MediaSource that can be embedded
// into a struct.
type Flow struct {
	// Start is called when the first receiver is added.
	Start func()

	// Stop is called when the last receiver is removed.
	Stop func()

	subscribers []chan []byte

	sync.Mutex
}

func (f *Flow) Subscribe(capacity int) <-chan []byte {
	f.Lock()
	defer f.Unlock()

	if capacity == 0 {
		panic("media.Flow: receiver capacity must be nonzero")
	}

	s := make(chan []byte, capacity)
	f.subscribers = append(f.subscribers, s)
	if f.Start != nil && len(f.subscribers) == 1 {
		f.Start()
	}
	return s
}

func (f *Flow) Unsubscribe(s <-chan []byte) error {
	f.Lock()
	defer f.Unlock()

	// Find and delete r from the receivers list.
	// See https://github.com/golang/go/wiki/SliceTricks
	for i, subscriber := range f.subscribers {
		if s == subscriber {
			subs := f.subscribers
			close(subs[i])
			subs[len(subs)-1], subs[i] = subs[i], subs[len(subs)-1]
			f.subscribers = subs[:len(subs)-1]
			break
		}
	}

	if f.Stop != nil && len(f.subscribers) == 0 {
		go f.Stop()
	}

	return nil
}

func (f *Flow) Write(p []byte) (n int, err error) {
	f.Lock()
	defer f.Unlock()

	for _, subscriber := range f.subscribers {
		select {
		case subscriber <- p:
			// Added slice reference to subscriber
		default:
			// Drop oldest byte slice, add newest
			<-subscriber
			subscriber <- p

			log.Warn("media.Flow: subscriber missed a buffer")
			// TODO: Keep per-subscriber count of missed buffers?
		}
	}

	return len(p), nil
}

func (f *Flow) Close() error {
	f.Lock()
	defer f.Unlock()

	if len(f.subscribers) > 0 {
		for _, subscriber := range f.subscribers {
			close(subscriber)
			for len(subscriber) > 0 {
				<-subscriber // Drain
			}
		}
		f.subscribers = nil
	}

	return nil
}
