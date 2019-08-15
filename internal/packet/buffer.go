package packet

import "sync/atomic"

/*
A SharedBuffer represents a read-only byte buffer that may be accessed
concurrently from multiple goroutines. When a SharedBuffer is passed to a
consumer function, the consumer should process the bytes and Release() the
buffer as quickly as possible. If the bytes cannot be processed quickly, the
consumer should make a copy, Release(), then continue processing its local copy.

Sharing is managed by reference counting. Hold() increments the reference count
by 1, Release() decrements it by 1. The done function is called when the count
reaches 0.

Example usage:

	func consumer(buf *SharedBuffer) {
		defer buf.Release() // Ensure the shared buffer will be released.
		data := buf.Bytes()
		// Process data...
	}

	func producer() {
		data := generateData()
		doneCh := make(chan struct{})
		buf := NewSharedBuffer(data, len(consumers), func() { close(doneCh) })
		for _, consume := consumers {
			go consume(buf)
		}
		// Wait for all consumers to finish.
		<-doneCh
	}

The goal is to avoid extraneous allocations/copies when a potentially large
byte buffer needs to be consumed by multiple goroutines.
*/
type SharedBuffer struct {
	data []byte

	count int32
	done  func()
}

func NewSharedBuffer(data []byte, count int, done func()) *SharedBuffer {
	return &SharedBuffer{data, int32(count), done}
}

// Bytes returns the underlying byte buffer.
func (buf *SharedBuffer) Bytes() []byte {
	return buf.data
}

// Increments the hold count.
func (buf *SharedBuffer) Hold() {
	atomic.AddInt32(&buf.count, 1)
}

// Decrements the hold count. When the hold count reaches zero, the underlying
// byte buffer will be released.
func (buf *SharedBuffer) Release() {
	if buf == nil {
		return
	}
	newCount := atomic.AddInt32(&buf.count, -1)
	if newCount == 0 {
		if buf.done != nil {
			buf.done()
		}
		buf.data = nil
	}
}
