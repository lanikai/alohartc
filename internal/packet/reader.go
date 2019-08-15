package packet

import (
	"fmt"
)

type Reader struct {
	buffer []byte
	offset int
}

func NewReader(buffer []byte) *Reader {
	return &Reader{buffer, 0}
}

func (r *Reader) ReadByte() byte {
	v := r.buffer[r.offset]
	r.offset++
	return v
}

func (r *Reader) ReadUint16() uint16 {
	v := networkOrder.Uint16(r.buffer[r.offset:])
	r.offset += 2
	return v
}

func (r *Reader) ReadUint24() uint32 {
	v := uint32(r.ReadByte()) << 16
	v |= uint32(r.ReadByte()) << 8
	v |= uint32(r.ReadByte())
	return v
}

func (r *Reader) ReadUint32() uint32 {
	v := networkOrder.Uint32(r.buffer[r.offset:])
	r.offset += 4
	return v
}

func (r *Reader) ReadSlice(n int) []byte {
	v := r.buffer[r.offset : r.offset+n]
	r.offset += n
	return v
}

func (r *Reader) ReadString(n int) string {
	return string(r.ReadSlice(n))
}

func (r *Reader) Skip(n int) {
	r.offset += n
}

// Discard bytes up to the next multiple of width, e.g. Align(4) skips ahead
// until the next aligned 4-byte boundary.
func (r *Reader) Align(width int) {
	r.offset = width * ((r.offset + width - 1) / width)
}

func (r *Reader) ReadRemaining() []byte {
	v := r.buffer[r.offset:]
	r.offset += len(v)
	return v
}

// Return the number of bytes left in the buffer.
func (r *Reader) Remaining() int {
	return len(r.buffer) - r.offset
}

func (r *Reader) CheckRemaining(needed int) error {
	if r.Remaining() < needed {
		return fmt.Errorf("%d bytes remaining, %d needed", r.Remaining(), needed)
	}
	return nil
}
