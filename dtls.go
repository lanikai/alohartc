package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"time"
	"unsafe"
)

const (
	// Handshake types
	HelloRequestHandshakeType = 0
	ClientHelloHandshakeType = 1
	ServerHelloHandshakeType = 2
	HelloVerifyRequestHandshakeType = 3
	CertificateHandshakeType = 11
	ServerKeyExchangeHandshakeType = 12
	CertificateRequestHandshakeType = 13
	ServerHelloDoneHandshakeType = 14
	CertificateVerifyHandshakeType = 15
	ClientKeyExchangeHandshakeType = 16
	FinishedHandshakeType = 20

	NullCompressionMethod = 0
)

// Cipher suites
var TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA = CipherSuite{ 0xC0, 0x13 }
var TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 = CipherSuite{ 0xC0, 0x2F }

type HandshakeType uint

type CipherSuite [2]uint8

type CompressionMethod uint8

type Handshake struct {
	MessageType HandshakeType
	Body interface{}
}

type ClientHello struct {
	ClientVersion ProtocolVersion
	Random
	SessionID []byte
	Cookie []byte
	CipherSuites []CipherSuite
	CompressionMethods []CompressionMethod
}

type ProtocolVersion struct {
	Major uint8 // 3 for DTLS 1.2
	Minor uint8 // 1 for DTLS 1.0; 3 for DTLS 1.2
}

func (pv *ProtocolVersion) MarshalBinary() ([]byte, error) {
	b := make([]byte, 2, 2)

	b[0] = pv.Major
	b[1] = pv.Minor

	return b, nil
}

type Random struct {
	Time time.Time
	Bytes [28]byte
}

func NewRandom() Random {
	random := Random{
		Time: time.Now(),
	}

	rand.Read(random.Bytes[:])

	return random
}

func (random *Random) MarshalBinary() ([]byte, error) {
	b := make([]byte, 32, 32)

	binary.BigEndian.PutUint32(b[0:4], uint32(random.Time.Unix()))
	copy(b[4:32], random.Bytes[:])

	return b, nil
}

func NewClientHello() *Handshake{
	handshake := &Handshake{
		MessageType: ClientHelloHandshakeType,
		Body: ClientHello{
			ClientVersion: ProtocolVersion{
				Major: 254,
				Minor: 253, // DTLS 1.0
			},
			Random: NewRandom(),
			SessionID: nil,
			CipherSuites: []CipherSuite{
				// Only cipher suite required for DTLS 1.0
				TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			},
			CompressionMethods: []CompressionMethod{
				NullCompressionMethod,
			},
		},
	}

	return handshake
}

func (ch *ClientHello) MarshalBinary() ([]byte, error) {
	offset := 0

	sessionIDLength := len(ch.SessionID)
	cookieLength := len(ch.Cookie)
	numCipherSuites := len(ch.CipherSuites)
	numCompressionMethods := len(ch.CompressionMethods)

	variableLength := sessionIDLength +
		cookieLength +
		2 * numCipherSuites +
		numCompressionMethods

	b := make([]byte, 39 + variableLength, 39 + variableLength)

	// Client protocol version
	if cv, err := ch.ClientVersion.MarshalBinary(); err != nil {
		return nil, err
	} else {
		copy(b[0:2], cv)
	}

	// Random
	if r, err := ch.Random.MarshalBinary(); err != nil {
		return nil, err
	} else {
		copy(b[2:34], r)
	}

	// Session ID
	b[34] = uint8(sessionIDLength)
	copy(b[35:], ch.SessionID)
	offset += sessionIDLength

	// Cookie
	b[35 + offset] = uint8(cookieLength)
	copy(b[36 + offset:], ch.Cookie)
	offset += cookieLength

	// Cipher suites
	binary.BigEndian.PutUint16(b[36 + offset:], uint16(2 * numCipherSuites))
	for _, cs := range ch.CipherSuites {
		copy(b[38 + offset:], cs[:])
		offset += int(unsafe.Sizeof(cs))
	}

	// Compression methods
	b[38 + offset] = uint8(numCompressionMethods)
	for i, cm := range ch.CompressionMethods {
		b[39 + offset + i] = uint8(cm)
	}

	return b, nil
}

func (h *Handshake) MarshalBinary() ([]byte, error) {
	var err error
	var body []byte

	switch h.Body.(type) {
	case ClientHello:
		clientHello := h.Body.(ClientHello)
		body, err = clientHello.MarshalBinary()
		if err != nil {
			return nil, err
		}
	}

	length := len(body)

	b := make([]byte, 12 + length, 12 + length)

	// Go does not have a uint24 type. Hence write later offsets first and
	// overwrite overlapping values with earlier offsets.
	binary.BigEndian.PutUint32(b[8:], uint32(length))
	binary.BigEndian.PutUint32(b[5:], 0)
	binary.BigEndian.PutUint16(b[4:], 0)
	binary.BigEndian.PutUint32(b[0:], uint32(length))
	b[0] = uint8(h.MessageType)

	copy(b[12:], body)

	return b, nil
}

type DTLSConn struct {
	conn net.Conn
}

func sendClientHello(w io.Writer) {
	handshake := NewClientHello()

	body, _ := handshake.MarshalBinary()

	length := len(body)

	b := make([]byte, 13 + length, 13 + length)

	b[0] = 22
	b[1] = 254 // DTLS 1.0
	b[2] = 255 // DTLS 1.0
	binary.BigEndian.PutUint64(b[3:], uint64(0))
	binary.BigEndian.PutUint16(b[3:], 0)
	binary.BigEndian.PutUint16(b[11:13], uint16(length))

	copy(b[13:], body)

	w.Write(b)
}

func DialDTLS(conn net.Conn) (DTLSConn, error) {
/*
        raddr, err := net.ResolveUDPAddr("udp", address)
        if err != nil {
                return DTLSConn{}, err
        }

        conn, err := net.DialUDP("udp", nil, raddr)
        if err != nil {
                return DTLSConn{}, err
        }
*/

	// Send ClientHello
	sendClientHello(conn)

	return DTLSConn{ conn }, nil
}

func (dtls *DTLSConn) Write([]byte) {
}

func (dtls *DTLSConn) Close() error {
	return dtls.conn.Close()
}
