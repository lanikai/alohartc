package srtp

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEncrypt(t *testing.T) {
	testMasterKey, _ := hex.DecodeString("E1F97A0D3E018BE0D64FA32C06DE4139")
	testMasterSalt, _ := hex.DecodeString("0EC675AD498AFEEBB6960B3AABE6")

	encipherContext, err := CreateContext(testMasterKey, testMasterSalt)
	if err != nil {
		t.Fail()
	}

	plaintext := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	ciphertext := []byte{
		0x7c, 0x64, 0x06, 0x03, 0xe8, 0x1d, 0x44, 0x0d,
		0xf2, 0x3d, 0xdb, 0xe5, 0xb0, 0x7f, 0x88, 0x7a,
	}

	testMsg := &rtpMsg{
		payloadType:    1,
		timestamp:      2,
		marker:         false,
		csrc:           []uint32{},
		ssrc:           12345678,
		sequenceNumber: 1,
		payload:        plaintext,
	}

	encipherContext.encrypt(testMsg)
	if 0 != bytes.Compare(testMsg.payload[0:len(ciphertext)], ciphertext) {
		t.Fail()
	}
}

func TestDecrypt(t *testing.T) {
	testMasterKey, _ := hex.DecodeString("E1F97A0D3E018BE0D64FA32C06DE4139")
	testMasterSalt, _ := hex.DecodeString("0EC675AD498AFEEBB6960B3AABE6")

	decipherContext, err := CreateContext(testMasterKey, testMasterSalt)
	if err != nil {
		t.Fail()
	}

	plaintext := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	ciphertext := []byte{
		0x7c, 0x64, 0x06, 0x03, 0xe8, 0x1d, 0x44, 0x0d,
		0xf2, 0x3d, 0xdb, 0xe5, 0xb0, 0x7f, 0x88, 0x7a,
	}

	testMsg := &rtpMsg{
		payloadType:    1,
		timestamp:      2,
		marker:         false,
		csrc:           []uint32{},
		ssrc:           12345678,
		sequenceNumber: 1,
		payload:        ciphertext,
	}

	decipherContext.decrypt(testMsg)
	if 0 != bytes.Compare(testMsg.payload[0:len(plaintext)], plaintext) {
		t.Fail()
	}
}
