// +build aes_stdlib

package aes

import (
	aes_ "crypto/aes"
	"crypto/cipher"
)

type BlockCipher struct {
	b cipher.Block
}

func NewCipher(key []byte) (BlockCipher, error) {
	b, err := aes_.NewCipher(key)
	return BlockCipher{b}, err
}

func (block *BlockCipher) CounterMode(iv []byte) cipher.Stream {
	return cipher.NewCTR(block.b, iv)
}

func (block *BlockCipher) CounterModeEncrypt(iv []byte, data []byte) error {
	cipher.NewCTR(block.b, iv).XORKeyStream(data, data)
	return nil
}
