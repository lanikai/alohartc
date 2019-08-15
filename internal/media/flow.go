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

	receivers []chan *packet.SharedBuffer

	sync.Mutex
}

func (f *Flow) AddReceiver(capacity int) <-chan *packet.SharedBuffer {
	f.Lock()
	defer f.Unlock()

	if capacity == 0 {
		panic("media.Flow: receiver capacity must be nonzero")
	}

	r := make(chan *packet.SharedBuffer, capacity)
	f.receivers = append(f.receivers, r)
	if f.Start != nil && len(f.receivers) == 1 {
		go f.Start()
	}
	return r
}

func (f *Flow) RemoveReceiver(r <-chan *packet.SharedBuffer) {
	f.Lock()
	defer f.Unlock()

	// Find and delete r from the receivers list.
	// See https://github.com/golang/go/wiki/SliceTricks
	for i := range f.receivers {
		if f.receivers[i] == r {
			closeAndDrain(f.receivers[i])
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

func closeAndDrain(r chan *packet.SharedBuffer) {
	close(r)
	for buf := range r {
		buf.Release()
	}
}

func (f *Flow) Put(buf *packet.SharedBuffer) {
	f.Lock()
	defer f.Unlock()

	for _, r := range f.receivers {
		buf.Hold()
		select {
		case r <- buf:
		default:
			log.Warn("media.Flow: receiver missed a buffer")
			buf.Release()
		}
	}
}

// TODO: Should this be exported?
func (f *Flow) PutBuffer(data []byte, done func()) {
	f.Put(packet.NewSharedBuffer(data, 0, done))
}
