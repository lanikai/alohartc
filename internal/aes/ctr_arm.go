// +build arm

package aes

import (
	"crypto/cipher"
	"encoding/binary"
)

// ctrAble is implemented by cipher.Blocks that can provide an optimized
// implementation of CTR through the cipher.Stream interface.
// See crypto/cipher/ctr.go.
type ctrAble interface {
	NewCTR(iv []byte) cipher.Stream
}

// Assert that aesCipher implements the ctrAble interface.
var _ ctrAble = (*aesCipher)(nil)

// streamBufferSize is the number of bytes of encrypted counter values to cache.
const streamBufferSize = 32 * BlockSize

type aesctr struct {
	block   *aesCipher             // block cipher
	ctr     [2]uint64              // next value of the counter (big endian)
	buffer  []byte                 // buffer for the encrypted counter values
	storage [streamBufferSize]byte // array backing buffer slice
}

// NewCTR returns a Stream which encrypts/decrypts using the AES block
// cipher in counter mode. The length of iv must be the same as BlockSize.
func (c *aesCipher) NewCTR(iv []byte) cipher.Stream {
	if len(iv) != BlockSize {
		panic("cipher.NewCTR: IV length must equal block size")
	}
	var ac aesctr
	ac.block = c
	ac.ctr[0] = binary.BigEndian.Uint64(iv[0:]) // high bits
	ac.ctr[1] = binary.BigEndian.Uint64(iv[8:]) // low bits
	ac.buffer = ac.storage[:0]
	return &ac
}

func (c *aesctr) refill() {
	// Either 10, 12, or 14 rounds, corresponding to AES-128, -192, and -256.
	nr := len(c.block.enc)/4 - 1

	// Fill up the buffer with an incrementing count.
	c.buffer = c.storage[:streamBufferSize]
	c0, c1 := c.ctr[0], c.ctr[1]
	for i := 0; i < streamBufferSize; i += 16 {
		binary.BigEndian.PutUint64(c.buffer[i+0:], c0)
		binary.BigEndian.PutUint64(c.buffer[i+8:], c1)

		// Encrypt counter in place to produce the key stream.
		encryptBlockAsm(nr, &c.block.enc[0], &c.buffer[i], &c.buffer[i])

		// Increment counter in big endian: c0 is high, c1 is low
		c1++
		if c1 == 0 {
			c0++
		}
	}

	c.ctr[0], c.ctr[1] = c0, c1
}

// xorBytes xors the contents of a and b and places the resulting values into
// dst. If a and b are not the same length then the number of bytes processed
// will be equal to the length of shorter of the two. Returns the number of
// bytes processed.
func xorBytes(dst, a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}

	_ = dst[n-1]
	xorBytesAsm(&dst[0], &a[0], &b[0], n)
	return n
}

//go:noescape
func xorBytesAsm(dst, a, b *byte, n int)

func (c *aesctr) XORKeyStream(dst, src []byte) {
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	// TODO
	//if subtle.InexactOverlap(dst[:len(src)], src) {
	//	panic("crypto/cipher: invalid buffer overlap")
	//}
	for len(src) > 0 {
		if len(c.buffer) == 0 {
			c.refill()
		}
		n := xorBytes(dst, src, c.buffer)
		c.buffer = c.buffer[n:]
		src = src[n:]
		dst = dst[n:]
	}
}
