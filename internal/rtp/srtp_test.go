package rtp

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/lanikai/alohartc/internal/packet"
)

func TestEncryptRTP(t *testing.T) {
	masterKey := []byte("TopSecret128bits")
	masterSalt := []byte("SodiumChloride")
	crypto := newCryptoContext(masterKey, masterSalt)

	index := uint64(123456)
	hdr := rtpHeader{
		payloadType: 100,
		sequence:    uint16(index),
		timestamp:   55555555,
		ssrc:        0x1337d00d,
	}
	payload := []byte("abcdefghijklmnopqrstuvwxyz")

	// Write the RTP packet to the buffer.
	p := packet.NewWriterSize(512)
	hdr.writeTo(p)
	p.WriteSlice(payload)

	err := crypto.encryptAndSignRTP(p, &hdr, index)
	if err != nil {
		t.Error(err)
	}

	// p now contains an encrypted SRTP packet. Read it back and compare.
	payloadOut, err := crypto.verifyAndDecryptRTP(p.Bytes(), &hdr, index)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(payload, payloadOut) {
		t.Errorf("RTP encrypt/decrypt failure: %s", payloadOut)
	}
}

func TestEncryptRTCP(t *testing.T) {
	masterKey := []byte("TopSecret128bits")
	masterSalt := []byte("SodiumChloride")
	crypto := newCryptoContext(masterKey, masterSalt)

	ssrc := uint32(0x1337d00d)
	index := uint64(123456)
	hdr := rtcpHeader{
		packetType: 200,
		length:     30,
	}
	payload := []byte("abcdefghijklmnopqrstuvwxyz")

	// Write the RTCP packet to the buffer.
	p := packet.NewWriterSize(512)
	hdr.writeTo(p)
	p.WriteUint32(ssrc)
	p.WriteSlice(payload)

	err := crypto.encryptAndSignRTCP(p, index)
	if err != nil {
		t.Error(err)
	}

	// p now contains an encrypted SRTCP packet. Read it back and compare.
	payloadOut, indexOut, err := crypto.verifyAndDecryptRTCP(p.Bytes())
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(payload, payloadOut) {
		t.Errorf("RTCP encrypt/decrypt failure: %s", payloadOut)
	}
	if index != indexOut {
		t.Errorf("RTCP index mismatch: %d", indexOut)
	}
}

// AES-CM Test Vectors: https://tools.ietf.org/html/rfc3711#appendix-B.2
func TestAESCounterMode(t *testing.T) {
	sessionKey, _ := hex.DecodeString("2B7E151628AED2A6ABF7158809CF4F3C")
	sessionSalt, _ := hex.DecodeString("F0F1F2F3F4F5F6F7F8F9FAFBFCFD0000")
	encrypt := aesCounterMode(sessionKey, sessionSalt)

	// Encrypt a block of zeros to get the keystream.
	keystream := make([]byte, 1044512)
	encrypt(keystream, uint32(0), uint64(0))

	if !checkHex(keystream[0:48],
		"E03EAD0935C95E80E166B16DD92B4EB4"+
			"D23513162B02D0F72A43A2FE4A5F97AB"+
			"41E95B3BB0A2E8DD477901E4FCA894C0") {
		t.Errorf("incorrect keystream start: %02X", keystream[0:48])
	}
	if !checkHex(keystream[len(keystream)-48:],
		"EC8CDF7398607CB0F2D21675EA9EA1E4"+
			"362B7C3C6773516318A077D7FC5073AE"+
			"6A2CC3787889374FBEB4C81B17BA6C44") {
		t.Errorf("incorrect keystream end: %02X", keystream[len(keystream)-48:])
	}
}

// Key Derivation Test Vectors: https://tools.ietf.org/html/rfc3711#appendix-B.3
func TestDeriveKey(t *testing.T) {
	masterKey, _ := hex.DecodeString("E1F97A0D3E018BE0D64FA32C06DE4139")
	masterSalt, _ := hex.DecodeString("0EC675AD498AFEEBB6960B3AABE6")

	key := deriveKey(masterKey, masterSalt, 0, 0x00, 16)
	if !checkHex(key, "C61E7A93744F39EE10734AFE3FF7A087") {
		t.Errorf("incorrect derived key: %02X", key)
	}

	salt := deriveKey(masterKey, masterSalt, 0, 0x02, 14)
	if !checkHex(salt, "30CBBC08863D8C85D49DB34A9AE1") {
		t.Errorf("incorrect derived salt: %02X", salt)
	}

	authKey := deriveKey(masterKey, masterSalt, 0, 0x01, 94)
	if !checkHex(authKey,
		"CEBE321F6FF7716B6FD4AB49AF256A15"+
			"6D38BAA48F0A0ACF3C34E2359E6CDBCE"+
			"E049646C43D9327AD175578EF7227098"+
			"6371C10C9A369AC2F94A8C5FBCDDDC25"+
			"6D6E919A48B610EF17C2041E47403576"+
			"6B68642C59BBFC2F34DB60DBDFB2") {
		t.Errorf("incorrect derived auth key: %02X", authKey)
	}
}

func checkHex(value []byte, expectedHex string) bool {
	return hex.EncodeToString(value) == strings.ToLower(expectedHex)
}
