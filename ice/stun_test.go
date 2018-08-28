package ice

import (
	"log"
	"net"
	"testing"
)

func TestMessageIntegrity(t *testing.T) {
	password := "hello"
	transactionID := "0123456789AB"
	raddr := &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5678}

	msg := newStunBindingResponse(transactionID)
	msg.setXorMappedAddress(raddr)
	msg.addMessageIntegrity(password)
	log.Println(msg.String())
	log.Printf("%x\n", msg.Bytes())
}

func TestFingerprint(t *testing.T) {
	password := "hello"
	transactionID := "0123456789AB"
	raddr := &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5678}

	msg := newStunBindingResponse(transactionID)
	msg.setXorMappedAddress(raddr)
	msg.addMessageIntegrity(password)
	msg.addFingerprint()
	log.Println(msg.String())
	log.Printf("%x\n", msg.Bytes())
}
