package webrtc

import (
	"bytes"
	"testing"
)

func TestParseStunMessage(t *testing.T) {
    b := []byte{
        0x00, 0x01, 0x00, 0x4c, 0x21, 0x12, 0xa4, 0x42,
        0x56, 0x41, 0x66, 0x33, 0x5a, 0x49, 0x73, 0x4c,
        0x31, 0x64, 0x2f, 0x46, 0x00, 0x06, 0x00, 0x09,
        0x74, 0x6c, 0x47, 0x61, 0x3a, 0x6e, 0x33, 0x45,
        0x33, 0x00, 0x00, 0x00, 0xc0, 0x57, 0x00, 0x04,
        0x00, 0x01, 0x00, 0x0a, 0x80, 0x29, 0x00, 0x08,
        0x57, 0xfa, 0x3a, 0xdb, 0xb9, 0x81, 0x0a, 0xdd,
        0x00, 0x24, 0x00, 0x04, 0x6e, 0x7f, 0x1e, 0xff,
        0x00, 0x08, 0x00, 0x14, 0x16, 0xae, 0x21, 0xab,
        0x58, 0xa5, 0xba, 0x5f, 0x5d, 0x1d, 0xfe, 0xde,
        0xc5, 0x65, 0x52, 0xf5, 0x6f, 0x08, 0x60, 0x37,
        0x80, 0x28, 0x00, 0x04, 0x31, 0xfd, 0x4e, 0x69,
    }

	msg := parseStunMessage(b)
	if msg == nil {
		t.Error("Failed to parse STUN message")
	}
	t.Log("type:", msg.header.MessageType)
	t.Log("length:", msg.header.MessageLength)
	t.Logf("magic cookie: %#x", msg.header.MagicCookie)
	t.Log("transaction ID:", msg.header.TransactionID)
	t.Log("class:", msg.class)
	t.Log("method:", msg.method)
	t.Log("attributes:", msg.attributes)

	b2 := msg.Bytes()
	if !bytes.Equal(b, b2) {
		t.Errorf("Serialized STUN message not equal to original: %s", b2)
	}

	msg2, err := newStunMessage(msg.class, msg.method, msg.header.TransactionID[:])
	if err != nil {
		t.Error(err)
	}
	for _, attr := range msg.attributes {
		msg2.AddAttribute(attr.Type, attr.Value)
	}

	b3 := msg2.Bytes()
	if !bytes.Equal(b, b3) {
		t.Errorf("Reconstructed STUN message not equal to original: %s", b3)
	}
}

func TestNewStunMessage(t *testing.T) {
	tid := []byte("0123456789ab")
	msg, err := newStunMessage(stunRequestClass, 0, tid)
	if err != nil {
		t.Errorf("Failed to create STUN message: %s", err)
	}

	msg2 := parseStunMessage(msg.Bytes())
	if msg2 == nil {
		t.Error("Failed to parse STUN message")
	}
	if msg.header != msg2.header {
		t.Errorf("Parsed STUN header not equal to original")
	}
}

func TestPad4(t *testing.T) {
	vals := []uint16{ 0, 1, 2, 3, 4, 5, 6, 7, 8, 9 }
	answers := []int{ 0, 3, 2, 1, 0, 3, 2, 1, 0, 3 }
	for i, val := range vals {
		if pad4(val) != answers[i] {
			t.Errorf("pad4(%d) == %d != %d", val, pad4(val), answers[i])
		}
	}
}
