// MIT License
//
// Copyright (c) 2018 Pions
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Modification and extensions:
// Copyright (c) 2019 Lanikai Labs. All rights reserved.

package srtp

import (
	"crypto/cipher"
	"encoding/binary"
	"errors"
)

// See https://tools.ietf.org/html/rfc3711#section-3.4 for SRTCP packet struct
// see https://tools.ietf.org/html/rfc3711#section-4.1.1 for AES CTR details
func (c *Context) DecipherRTCP(deciphered, enciphered []byte) ([]byte, error) {
	if len(enciphered) < 8+authTagSize+srtcpIndexSize {
		return nil, errors.New("srtcp packet too short")
	}

	// Allocation optimization
	out := allocateIfMismatch(deciphered, enciphered)

	// Compute offset of packet tail
	tailOffset := len(enciphered) - (authTagSize + srtcpIndexSize)
	out = out[0:tailOffset]

	// Check whether enciphered
	if isEnciphered := enciphered[tailOffset] >> 7; isEnciphered == 0 {
		return out, nil
	}

	// Index is an up-counter. Each packet is one more than the last.
	index := binary.BigEndian.Uint32(enciphered[tailOffset:]) & 0x7fffffff

	// Source identifier
	ssrc := binary.BigEndian.Uint32(out[4:])

	// Decipher in-place
	stream := cipher.NewCTR(
		c.srtcpBlock,
		c.generateCounter(
			uint16(index&0xffff),
			index>>16,
			ssrc,
			c.srtcpSessionSalt,
		),
	)
	stream.XORKeyStream(out[8:], out[8:])

	return out, nil
}
