package aes

import (
	_ "unsafe"
)

//go:linkname inexactOverlap crypto/internal/subtle.InexactOverlap
func inexactOverlap(x, y []byte) bool
