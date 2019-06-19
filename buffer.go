//////////////////////////////////////////////////////////////////////////////
//
// Buffer for MediaSource implementing io.Reader and io.WriteCloser
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"io"
)

type Buffer struct {
	ch   chan []byte
	dead chan struct{}
}

func NewBuffer() *Buffer {
	return &Buffer{
		ch:   make(chan []byte, 32),
		dead: make(chan struct{}),
	}
}

// Read next frame from the buffer. Blocks until either a frame is available or
// the buffer is closed.
func (b *Buffer) Read(p []byte) (int, error) {
	select {
	case <-b.dead:
		return 0, io.EOF
	case buf := <-b.ch:
		n := copy(p, buf)
		if n < len(buf) {
			return n, io.ErrShortBuffer
		}
		return n, nil
	}
}

// Write frame into buffer. Blocks until able to write to buffer (i.e. a reader
// is listening and buffer has space).
func (b *Buffer) Write(p []byte) (int, error) {
	// Copy p into a temporary buffer, that we can send across the channel.
	buf := append([]byte(nil), p...)

	select {
	case b.ch <- buf:
		return len(buf), nil
	case <-b.dead:
		return 0, io.ErrClosedPipe
	}
}

// Close the buffer. Any further reads will return io.EOF. Further writes will
// panic.
func (b *Buffer) Close() error {
	select {
	case <-b.dead:
	default:
		close(b.dead)
	}
	return nil
}
