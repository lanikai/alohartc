package media

import (
	"sync"

	"github.com/lanikai/alohartc/internal/packet"
)

// Flow is a concrete implementation of Source that can be embedded into a
// struct.
type Flow struct {
	// Start is called when the first receiver is added.
	Start func()

	// Stop is called when the last receiver is removed.
	Stop func()

	receivers []*flowReceiver

	sync.Mutex
}

func (f *Flow) AddReceiver(capacity int) Receiver {
	f.Lock()
	defer f.Unlock()

	if capacity == 0 {
		panic("media.Flow: receiver capacity must be nonzero")
	}

	r := &flowReceiver{
		ch: make(chan *packet.SharedBuffer, capacity),
	}
	f.receivers = append(f.receivers, r)
	if f.Start != nil && len(f.receivers) == 1 {
		f.Start()
	}
	return r
}

func (f *Flow) RemoveReceiver(r Receiver) {
	f.Lock()
	defer f.Unlock()

	// Find and delete r from the receivers list.
	// See https://github.com/golang/go/wiki/SliceTricks
	for i := range f.receivers {
		if f.receivers[i] == r {
			f.receivers[i].closeAndDrain()

			n := len(f.receivers)
			copy(f.receivers[i:], f.receivers[i+1:])
			f.receivers[n-1] = nil
			f.receivers = f.receivers[:n-1]
			break
		}
	}

	if f.Stop != nil && len(f.receivers) == 0 {
		go f.Stop()
	}
}

func (f *Flow) Put(buf *packet.SharedBuffer) error {
	f.Lock()
	defer f.Unlock()

	for _, r := range f.receivers {
		buf.Hold()
		select {
		case r.ch <- buf:
		default:
			log.Warn("media.Flow: receiver missed a buffer")
			// TODO: Keep per-receiver count of missed buffers?
			buf.Release()
		}
	}
	buf.Release()
	return nil
}

// TODO: Do we need this?
func (f *Flow) PutBuffer(data []byte, done func()) error {
	return f.Put(packet.NewSharedBuffer(data, 1, done))
}

func (f *Flow) Shutdown(cause error) {
	f.Lock()
	defer f.Unlock()

	if len(f.receivers) > 0 {
		for _, r := range f.receivers {
			r.err = cause
			r.closeAndDrain()
		}
		f.receivers = nil
	}

}

type flowReceiver struct {
	ch  chan *packet.SharedBuffer
	err error
}

func (r *flowReceiver) Buffers() <-chan *packet.SharedBuffer {
	return r.ch
}

func (r *flowReceiver) Err() error {
	return r.err
}

func (r *flowReceiver) closeAndDrain() {
	close(r.ch)
	for buf := range r.ch {
		buf.Release()
	}
}
