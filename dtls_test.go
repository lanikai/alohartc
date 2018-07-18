package webrtc

import (
	"encoding/hex"
	"log"
	"testing"
)

func TestNewClientHello(t *testing.T) {
	ch := newClientHello().marshal()

	log.Println(hex.EncodeToString(ch))
}
