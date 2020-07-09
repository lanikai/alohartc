// +build !arm

// Use the Go standard library implementation of AES for everything except arm.

package aes

import (
	stdaes "crypto/aes"
)

const BlockSize = 16

var NewCipher = stdaes.NewCipher
