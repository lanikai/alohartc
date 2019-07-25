package media

import "sync/atomic"

/*
A SharedBuffer represents a read-only byte buffer that may be accessed
concurrently from multiple goroutines. When a SharedBuffer is passed to a
consumer function, the consumer should process the bytes and Release() the
buffer as quickly as possible. If the bytes cannot be processed quickly, the
consumer should make a copy, Release(), then continue processing its local copy.

Example usage:

	func consumer(buf *SharedBuffer) {
		defer buf.Release() // Ensure the shared buffer will be released.
		data := buf.Bytes()
		// Process data...
	}

	func producer() {
		data := generateData()
		var wg sync.WaitGroup
		buf := NewSharedBuffer(data, wg.Done)
		wg.Add(len(consumers))
		for , consume := consumers {
			consume(buf)
		}
		wg.Wait()
	}

The goal is to avoid extraneous allocations/copies when a potentially large
byte buffer needs to be consumed by multiple goroutines.
*/
type SharedBuffer struct {
	data []byte

	count   int32
	release func()
}

func NewSharedBuffer(data []byte, release func()) *SharedBuffer {
	return &SharedBuffer{data, 1, release}
}

// Bytes returns the underlying byte buffer. It must not be modified.
func (buf *SharedBuffer) Bytes() []byte {
	return buf.data
}

func (buf *SharedBuffer) Hold() {
	atomic.AddInt32(&buf.count, 1)
}

// Release the current loan on the shared buffer. This may be called multiple
// times, and is safe to call on a nil value.
func (buf *SharedBuffer) Release() {
	if buf == nil || buf.release == nil {
		return
	}
	newCount := atomic.AddInt32(&buf.count, -1)
	if newCount == 0 && buf.release != nil {
		buf.release()
	}
	buf.data = nil
}
