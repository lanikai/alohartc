package srtp

// Key Derivation Function (KDF) described in RFC 3711, section 4.3 (see
// https://tools.ietf.org/html/rfc3711#section-4.3). The KDF uses the DTLS
// master secret to derive keys used for secure RTP.

import (
	"crypto/aes"
	"encoding/binary"
)

const (
	labelSRTPEncryptionKey      = 0
	labelSRTPAuthenticationKey  = 1
	labelSRTPSaltingKey         = 2
	labelSRTCPEncryptionKey     = 3
	labelSRTCPAuthenticationKey = 4
	labelSRTCPSaltingKey        = 5
)

// key derivation function, as described in RFC 3711, section 4.3.
func kdf(masterKey, masterSalt []byte, index, rate uint, keyLen, ivLen int) (srtpKey, srtpSalt, srtcpKey, srtcpSalt []byte, err error) {
	// helper function
	fail := func(err error) ([]byte, []byte, []byte, []byte, error) {
		return nil, nil, nil, nil, err
	}

	// new AES-CM cipher instance
	cipher, err := aes.NewCipher(masterKey)
	if err != nil {
		return fail(err)
	}

	// compute key_id (see https://tools.ietf.org/html/rfc3711#section-4.3)
	keyId := make([]byte, 6)
	if rate != 0 {
		binary.BigEndian.PutUint64(keyId, uint64(index/rate)&((1<<48)-1))
	}

	// helper function
	derive := func(label uint8) []byte {
		x := make([]byte, 16)
		copy(x[0:], masterSalt)
		x[7] ^= label
		x[8] ^= keyId[0]
		x[9] ^= keyId[1]
		x[10] ^= keyId[2]
		x[11] ^= keyId[3]
		x[12] ^= keyId[4]
		x[13] ^= keyId[5]
		cipher.Encrypt(x, x)
		return x
	}

	// derive keys
	srtpKey = derive(labelSRTPEncryptionKey)[:keyLen]
	srtpSalt = derive(labelSRTPSaltingKey)[:ivLen]
	srtcpKey = derive(labelSRTCPEncryptionKey)[:keyLen]
	srtcpSalt = derive(labelSRTCPSaltingKey)[:ivLen]

	return
}
