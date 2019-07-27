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

// Read into the provided buffer. See io.Reader. Always returns nil error.
//func (r *Reader) Read(p []byte) (n int, err error) {
//	n = copy(p, r.buffer[r.offset:])
//	r.offset += n
//	return
//}

func (r *Reader) ReadSlice(n int) []byte {
	v := r.buffer[r.offset : r.offset+n]
	r.offset += n
	return v
}

func (r *Reader) Skip(n int) {
	r.offset += n
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
