//////////////////////////////////////////////////////////////////////////////
//
// Buffer for MediaSource implementing io.Reader and io.Writer
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

type Buffer struct {
	ch chan []byte
}

// NewBuffer returns a new buffer
func NewBuffer() *Buffer {
	return &Buffer{
		ch: make(chan []byte, 32),
	}
}

// Read next frame from the buffer. Blocks until a frame available.
func (b *Buffer) Read(p []byte) (int, error) {
	// TODO If len(p) < len(<-b.ch), tail of frame is lost
	return copy(p, <-b.ch), nil
}

// Write frame into buffer. Blocks until able to write to buffer (i.e. a reader
// is listening and buffer has space).
func (b *Buffer) Write(p []byte) {
	b.ch <- p
}
