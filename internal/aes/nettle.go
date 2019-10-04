// +build aes_nettle

package aes

// #include <nettle/aes.h>
// #include <nettle/ctr.h>
import "C"

import (
	"crypto/cipher"
	"fmt"
	"unsafe"
)

type BlockCipher struct {
	ctx C.struct_aes128_ctx
}

func NewCipher(key []byte) (block BlockCipher, err error) {
	if len(key) != 16 {
		err = fmt.Errorf("invalid AES-128 key length: %d", len(key))
		return
	}

	C.aes128_set_encrypt_key(&block.ctx, (*C.uchar)(&key[0]))
	return
}

func (block *BlockCipher) CounterMode(iv []byte) cipher.Stream {
	stream := new(aesCounterMode)
	stream.ctx = block.ctx
	copy(stream.ctr[:], iv)
	return stream
}

// Counter-mode context, like CTR_CTX(struct aes128_ctx, AES_BLOCK_SIZE).
type aesCounterMode struct {
	ctx C.struct_aes128_ctx
	ctr [BlockSize]byte
}

func (stream *aesCounterMode) XORKeyStream(dst, src []byte) {
	C.ctr_crypt(
		unsafe.Pointer(&stream.ctx),
		(*C.nettle_cipher_func)(C.nettle_aes128_encrypt),
		BlockSize,
		(*C.uchar)(&stream.ctr[0]),
		C.size_t(len(src)),
		(*C.uchar)(&dst[0]),
		(*C.uchar)(&src[0]))
}

func (block *BlockCipher) CounterModeEncrypt(iv []byte, data []byte) error {
	if len(iv) < BlockSize {
		return fmt.Errorf("invalid AES-CTR IV: %v", iv)
	}

	C.ctr_crypt(
		unsafe.Pointer(&block.ctx),
		(*C.nettle_cipher_func)(C.nettle_aes128_encrypt),
		BlockSize,
		(*C.uchar)(&iv[0]),
		C.size_t(len(data)),
		(*C.uchar)(&data[0]),
		(*C.uchar)(&data[0]))
	return nil
}
