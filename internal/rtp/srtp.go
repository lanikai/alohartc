package rtp

// srtp.go implements the Secure RTP profile (SRTP) from RFC 3711.
//
// See [RFC 3711](https://tools.ietf.org/html/rfc3711) and the modifications for
// reduced-size RTCP in [RFC 5506](https://tools.ietf.org/html/rfc5506).

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"hash"
	"sync"

	"github.com/lanikai/alohartc/internal/packet"
)

const (
	// Default SRTP key management parameters.
	// See https://tools.ietf.org/html/rfc3711#section-8.2
	authKeyLength    = 20 // n_a = 160 bits
	authTagLength    = 10 // n_tag = 80 bits
	encryptKeyLength = 16 // n_e = 128 bits
	saltKeyLength    = 14 // n_s = 112 bits

	// E-flag that gets combined with SRTCP index.
	eFlagMask = 1 << 31
)

// An encryptFunc encrypts an RTP/RTCP payload in place, using a unique
// cryptographic keystream for each combination of SSRC and index.
type encryptFunc func(payload []byte, ssrc uint32, index uint64)

// An authFunc computes the authentication tag for integrity-protected data.
type authFunc func(M []byte) []byte

// Cryptographic context for SRTP and SRTCP. (Note that in contrast to RFC 3711,
// the rollover counter and SRTCP index are *not* stored here; they must be
// maintained elsewhere, and passed in as parameters to all SRTP operations.)
type cryptoContext struct {
	encryptSRTP       encryptFunc
	encryptSRTCP      encryptFunc
	authenticateSRTP  authFunc
	authenticateSRTCP authFunc

	// TODO: Replay lists
}

func newCryptoContext(masterKey, masterSalt []byte) *cryptoContext {
	var (
		srtpEncryptKey  = deriveKey(masterKey, masterSalt, 0, 0x00, encryptKeyLength)
		srtpAuthKey     = deriveKey(masterKey, masterSalt, 0, 0x01, authKeyLength)
		srtpSaltKey     = deriveKey(masterKey, masterSalt, 0, 0x02, saltKeyLength)
		srtcpEncryptKey = deriveKey(masterKey, masterSalt, 0, 0x03, encryptKeyLength)
		srtcpAuthKey    = deriveKey(masterKey, masterSalt, 0, 0x04, authKeyLength)
		srtcpSaltKey    = deriveKey(masterKey, masterSalt, 0, 0x05, saltKeyLength)
	)
	return &cryptoContext{
		encryptSRTP:       defaultEncryptTransform(srtpEncryptKey, srtpSaltKey),
		encryptSRTCP:      defaultEncryptTransform(srtcpEncryptKey, srtcpSaltKey),
		authenticateSRTP:  defaultAuthTransform(srtpAuthKey),
		authenticateSRTCP: defaultAuthTransform(srtcpAuthKey),
	}
}

// Encrypt the payload of an RTP packet in place, then compute and append the
// authentication tag. p is the packet buffer, payloadStart is the offset to the
// RTP payload (i.e. just after the RTP header), ssrc is the packet's SSRC
// field, and index is the extended sequence number (i.e. ROC*2^16 + SEQ).
// See https://tools.ietf.org/html/rfc3711#section-3.1
// and https://tools.ietf.org/html/rfc3711#section-3.3
func (c *cryptoContext) encryptAndSignRTP(p *packet.Writer, hdr *rtpHeader, index uint64) error {
	// Encrypt the payload only.
	payloadStart := hdr.length()
	c.encryptSRTP(p.Bytes()[payloadStart:], hdr.ssrc, trunc(index, 48))

	// From https://tools.ietf.org/html/rfc3711#section-4.2:
	//   In the case of SRTP, M SHALL consist of the Authenticated Portion of
	//   the packet (as specified in Figure 1) concatenated with the ROC,
	//	 M = Authenticated Portion || ROC;
	// To compute the auth tag, temporarily write the ROC into the packet
	// buffer, then rewind and overwrite with the tag.
	p.WriteUint32(uint32(index >> 16)) // ROC is just the high bits of the index
	tag := c.authenticateSRTP(p.Bytes())
	p.Rewind(4)
	return p.WriteSlice(tag)
}

// Verify the auth tag of the SRTP packet, then decrypt and return the payload.
// This is the inverse of encryptAndSignRTP().
func (c *cryptoContext) verifyAndDecryptRTP(buf []byte, hdr *rtpHeader, index uint64) ([]byte, error) {
	payloadStart := hdr.length()
	tagStart := len(buf) - authTagLength
	if tagStart < 0 {
		return nil, errors.New("SRTP packet too short")
	}

	// Temporarily replace the first 4 bytes of the encoded auth tag with the
	// ROC in order to compute the expected auth tag. Then replace and compare.
	tmp := binary.BigEndian.Uint32(buf[tagStart:])
	binary.BigEndian.PutUint32(buf[tagStart:], uint32(index>>16)) // ROC
	tag := c.authenticateSRTP(buf[0 : tagStart+4])
	binary.BigEndian.PutUint32(buf[tagStart:], tmp)
	if !bytes.Equal(tag, buf[tagStart:]) {
		return nil, errors.New("SRTP integrity check failed")
	}

	// Now decrypt the payload. (Note the encryption transform is symmetric.)
	payload := buf[payloadStart:tagStart]
	c.encryptSRTP(payload, hdr.ssrc, trunc(index, 48))
	return payload, nil
}

// Encrypt the payload of an RTCP packet (reduced-size or compound) in place,
// then compute and append the SRTCP index and authentication tag.
// See https://tools.ietf.org/html/rfc3711#section-3.4
// and https://tools.ietf.org/html/rfc5506#section-3.4.3
func (c *cryptoContext) encryptAndSignRTCP(p *packet.Writer, index uint64) error {
	// Per RFC 5506, encrypt everything after the fixed RTCP header.
	buf := p.Bytes()
	ssrc := binary.BigEndian.Uint32(buf[4:8])
	c.encryptSRTCP(buf[8:], ssrc, trunc(index, 31))

	// From https://tools.ietf.org/html/rfc3711#section-4.2:
	//   in the case of SRTCP, M SHALL consist of the Authenticated Portion (as
	//   specified in Figure 2) only.
	// Append E || SRTCP index, then compute and append the auth tag.
	p.WriteUint32(eFlagMask | uint32(index))
	tag := c.authenticateSRTCP(p.Bytes())
	return p.WriteSlice(tag)
}

// Verify the auth tag of the SRTCP packet, then decrypt and return the packet
// along with the SRTCP index. This is the inverse of encryptAndSignRTCP().
func (c *cryptoContext) verifyAndDecryptRTCP(buf []byte) ([]byte, uint64, error) {
	tagStart := len(buf) - authTagLength
	indexStart := tagStart - 4
	if indexStart < 0 {
		return nil, 0, errors.New("SRTCP packet too short")
	}

	// Verify the auth tag.
	tag := c.authenticateSRTCP(buf[0:tagStart])
	if !bytes.Equal(tag, buf[tagStart:]) {
		return nil, 0, errors.New("SRTCP integrity check failed")
	}

	// Extract the SRTCP index.
	index := uint64(binary.BigEndian.Uint32(buf[indexStart:]))
	if index&eFlagMask == 0 {
		// E-flag is 0. Packet is not encrypted.
		return buf[8:indexStart], index, nil
	}
	index &^= eFlagMask

	// Now decrypt the payload. (Note the encryption transform is symmetric.)
	ssrc := binary.BigEndian.Uint32(buf[4:8])
	payload := buf[8:indexStart]
	c.encryptSRTCP(payload, ssrc, index)
	return payload, index, nil
}

// SRTP key derivation algorithm.
//  * r = index DIV key_derivation_rate is the 48-bit packet index divided by
//    the key derivation rate (or 0 if the rate is 0).
//  * label indicates which type of key to produce.
//  * n is the length of the output key in bytes.
// See https://tools.ietf.org/html/rfc3711#section-4.3
func deriveKey(masterKey, masterSalt []byte, r uint64, label byte, n int) []byte {
	// From https://tools.ietf.org/html/rfc3711#section-4.3:
	//   x = key_id XOR master_salt,
	// where
	//   key_id = <label> || r.
	// Then (https://tools.ietf.org/html/rfc3711#section-4.3.3) the IV for key
	// derivation is x*2^16. Pictorally, this looks like:
	//   xxxxxxxxxxxxxx00  <- salt (112 bits = 14 bytes)
	//   0000000x00000000  <- label
	//   00000000xxxxxx00  <- r
	x := append([]byte(nil), masterSalt...)

	// XOR with r, if necessary.
	if r > 0 {
		xor64(x[len(x)-8:], trunc(r, 48))
	}

	// Then XOR with <label>.
	x[len(x)-7] ^= label

	// Produce the stream cipher PRF_n(k_master, x).
	prf := defaultPRF(masterKey, x)

	// The derived key then comes from the PRF keystream, which we get by
	// XOR'ing with zeros.
	key := make([]byte, n)
	prf.XORKeyStream(key, key)
	return key
}

// The default PRF for SRTP is AES-CM.
// https://tools.ietf.org/html/rfc3711#section-4.3.3
func defaultPRF(masterKey, x []byte) cipher.Stream {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		panic(err)
	}
	if len(x) != aes.BlockSize {
		// IV equal to (x*2^16)
		x = padRight(x, aes.BlockSize)
	}
	return cipher.NewCTR(block, x)
}

// An encryptTransform specifies how the session key and salt are used to
// produce cryptographic keystreams.
type encryptTransform func(key, salt []byte) encryptFunc

// AES in counter mode (the default encryption transform for SRTP).
// See https://tools.ietf.org/html/rfc3711#section-4.1.1
func aesCounterMode(key, salt []byte) encryptFunc {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // invalid key size
	}
	// Reuse IV byte slices, to reduce heap allocations.
	ivPool := sync.Pool{
		New: func() interface{} {
			return make([]byte, aes.BlockSize)
		},
	}

	return func(payload []byte, ssrc uint32, index uint64) {
		iv := ivPool.Get().([]byte)
		defer ivPool.Put(iv)

		// From https://tools.ietf.org/html/rfc3711#section-4.1.1:
		//   The 128-bit integer value IV SHALL be defined by the SSRC, the SRTP
		//   packet index i, and the SRTP session salting key k_s, as below.
		//       IV = (k_s * 2^16) XOR (SSRC * 2^64) XOR (i * 2^16)
		//
		// Pictorally, this looks like:
		//   xxxxxxxxxxxxxx00  <- salt (112 bits = 14 bytes)
		//   0000xxxx00000000  <- SSRC (32 bits = 4 bytes)
		//   00000000xxxxxx00  <- index (48 bits = 6 bytes)
		copy(iv, salt)
		clearBytes(iv[len(salt):])
		xor32(iv[4:], ssrc)
		xor64(iv[6:], index)

		cipher.NewCTR(block, iv).XORKeyStream(payload, payload)
	}
}

// For illustrative purposes only, not used by this package.
// See https://tools.ietf.org/html/rfc3711#section-4.1.3
func nullCipher(key, salt []byte) encryptFunc {
	// No-op encryption.
	return func(payload []byte, ssrc uint32, index uint64) {}
}

// TODO: Support other transforms (e.g. AES-GCM, RFC 7714).
var defaultEncryptTransform = aesCounterMode

// An authTransform specifies how the auth key is used to produce cryptographic
// hashes of RTP/RTCP message contents.
type authTransform func(authKey []byte) authFunc

// HMAC-SHA1 (the default authentication transform for SRTP).
// See https://tools.ietf.org/html/rfc3711#section-4.2
func hmacSHA1(authKey []byte) authFunc {
	// A pool of reusable HMAC-SHA1 hash instances, to reduce heap allocations.
	hashPool := sync.Pool{
		New: func() interface{} {
			return hmac.New(sha1.New, authKey)
		},
	}
	return func(M []byte) []byte {
		mac := hashPool.Get().(hash.Hash)
		mac.Write(M)
		tag := mac.Sum(nil)[0:authTagLength]

		mac.Reset()
		hashPool.Put(mac)
		return tag
	}
}

var defaultAuthTransform = hmacSHA1
