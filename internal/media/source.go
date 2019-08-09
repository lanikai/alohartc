package media

import (
	"sync"
)

/*
A Source is a stream of media data that can have multiple consumers. The media
data is chunked into packets (which may represent discrete video frames, or
spans of multiple audio frames). Consumer functions request a "receiver"
channel, to which the Source sends packets. Each packet is delivered as a
*SharedBuffer instance, which the consuming function must process and then
release.

The Source interface represents only the consumer-facing side of a media stream;
it makes no assumptions about how the data is produced. Nor does it describe the
nature of the data packets being delivered. It merely provides an interface for
common behavior between AudioSource and VideoSource, which extend Source.

Example usage:

	var src Source = ...
	r := src.AddReceiver(4)
	defer src.RemoveReceiver(r)
	for {
		buf, more := <-r
		if !more {
			break
		}
		// Do something with buf.Bytes(), then call buf.Release()
	}

*/
type Source interface {
	// AddReceiver creates a new receiver channel r, and starts passing incoming
	// data buffers to it. The source will not block sending to r, so the
	// capacity must be sufficient to keep up with the rate of incoming data.
	// (In particular, capacity must be > 0.) The channel may be closed if the
	// source is interrupted.
	//
	// Callers must ensure that the receiver is removed when processing is
	// complete (e.g. a defer statement immediately following AddReceiver()).
	AddReceiver(capacity int) (r <-chan *SharedBuffer)

	// RemoveReceiver tells the source to stop passing data buffers to r. Upon
	// return, it is guaranteed r will not receive any more data.
	RemoveReceiver(r <-chan *SharedBuffer)
}

// An implementation of Source that can be embedded into a struct.
type baseSource struct {
	sync.Mutex
	receivers []chan *SharedBuffer

	// start is called when the first receiver is added.
	start func()

	// stop is called when the last receiver is removed.
	stop func()
}

func (s *baseSource) AddReceiver(capacity int) <-chan *SharedBuffer {
	s.Lock()
	defer s.Unlock()

	if capacity == 0 {
		panic("media.Source: receiver capacity must be nonzero")
	}

	r := make(chan *SharedBuffer, capacity)
	s.receivers = append(s.receivers, r)
	if s.start != nil && len(s.receivers) == 1 {
		go s.start()
	}
	return r
}

func (s *baseSource) RemoveReceiver(r <-chan *SharedBuffer) {
	s.Lock()
	defer s.Unlock()

	// Find and delete r from the receivers list.
	// See https://github.com/golang/go/wiki/SliceTricks
	for i := range s.receivers {
		if s.receivers[i] == r {
			closeAndDrain(s.receivers[i])
			n := len(s.receivers)
			copy(s.receivers[i:], s.receivers[i+1:])
			s.receivers[n-1] = nil
			s.receivers = s.receivers[:n-1]
			break
		}
	}

	if s.stop != nil && len(s.receivers) == 0 {
		go s.stop()
	}
}

func closeAndDrain(r chan *SharedBuffer) {
	close(r)
	for buf := range r {
		buf.Release()
	}
}

// TODO: Should this be exported?
func (s *baseSource) putBuffer(data []byte, done func()) {
	s.Lock()
	defer s.Unlock()

	buf := NewSharedBuffer(data, len(s.receivers), done)
	for _, r := range s.receivers {
		select {
		case r <- buf:
		default:
			log.Warn("Source receiver missed a buffer")
			buf.Release()
		}
	}
}
