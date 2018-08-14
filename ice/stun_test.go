package ice

import (
	"log"
	"net"
	"testing"
)

func TestFingerprint(t *testing.T) {
	password := "hello"
	transactionID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	raddr := &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5678}

	msg := newStunBindingResponse(transactionID)
	msg.setXorMappedAddress(raddr)
	msg.addMessageIntegrity(password)
	msg.addFingerprint()
	log.Println(msg.String())
	log.Printf("%x\n", msg.Bytes())
}
