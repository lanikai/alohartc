package rtp

// Convenience functions for dealing with RTP packet formats. For example, the
// first byte of the RTP packet header:
//    0 1 2 3 4 5 6 7
//   +-+-+-+-+-+-+-+-+
//	 |V=2|P|X|  CC   |
//	 +-+-+-+-+-+-+-+-+
// can be parsed with
//    V, P, X, CC := splitByte2114(header[0])
// and put back together with
//    header[0] = joinByte2114(V, P, X, CC)

//   0 1 2 3 4 5 6 7
//   a a b c d d d d
func splitByte2114(v byte) (a2 byte, b1 bool, c1 bool, d4 byte) {
	a2 = v >> 6
	b1 = ((v >> 5) & 0x01) == 1
	c1 = ((v >> 4) & 0x01) == 1
	d4 = v & 0x0f
	return
}

// Inverse of splitByte2114.
func joinByte2114(a2 byte, b1 bool, c1 bool, d4 byte) byte {
	v := (a2 << 6) | (d4 & 0x0f)
	if b1 {
		v |= 0x20
	}
	if c1 {
		v |= 0x10
	}
	return byte(v)
}

// Split a byte into the first 2 bits, the next bit, and the remaining 5 bits.
func splitByte215(v byte) (a2 byte, b1 bool, c5 byte) {
	a2 = v >> 6
	b1 = ((v >> 5) & 0x01) == 1
	c5 = v & 0x1f
	return
}

func joinByte215(a2 byte, b1 bool, c5 byte) byte {
	v := (a2 << 6) | (c5 & 0x1f)
	if b1 {
		v |= 0x20
	}
	return v
}

// Split a byte into the first bit, the next 2 bits, and the remaining 5 bits.
func splitByte125(v byte) (a1 bool, b2 byte, c5 byte) {
	a1 = (v >> 7 & 0x01) == 1
	b2 = v >> 5 & 0x11
	c5 = v & 0x1f
	return
}

func joinByte125(a1 bool, b2 byte, c5 byte) byte {
	v := (b2 & 0x11 << 5) | (c5 & 0x1f)
	if a1 {
		v |= 0x80
	}
	return v
}

// Split a byte into the first bit and the remaining 7 bits.
// E.g. for the second byte of the RTP packet header:
//    0 1 2 3 4 5 6 7
//   +-+-+-+-+-+-+-+-+
//	 |M|     PT      |
//	 +-+-+-+-+-+-+-+-+
func splitByte17(v byte) (a1 bool, b7 byte) {
	a1 = (v >> 7) == 1
	b7 = v & 0x7f
	return
}

func joinByte17(b1 bool, b7 byte) byte {
	v := b7 & 0x7f
	if b1 {
		v |= 0x80
	}
	return byte(v)
}

// Truncate a 64-bit value to its lowest n bits.
func trunc(v uint64, n uint8) uint64 {
	return v & ((1 << n) - 1)
}

// XOR the bytes of a buffer with the given value.
func xor32(buf []byte, v uint32) {
	buf[0] ^= byte(v >> 24)
	buf[1] ^= byte(v >> 16)
	buf[2] ^= byte(v >> 8)
	buf[3] ^= byte(v)
}

// XOR the bytes of a buffer with the given value.
func xor64(buf []byte, v uint64) {
	xor32(buf[0:4], uint32(v>>32))
	xor32(buf[4:8], uint32(v))
}

// Zero out bytes in a slice. The compiler will optimize this down to a single
// `memclr` operation (https://github.com/golang/go/issues/5373).
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Pad a byte slice with zeros on the right, up to the desired size.
// See https://github.com/golang/go/wiki/SliceTricks#extend
func padRight(b []byte, desiredSize int) []byte {
	n := len(b)
	if n < desiredSize {
		b = append(b, make([]byte, desiredSize-n)...)
	}
	return b
}
