package webrtc

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"
	"unsafe"
)

/////////////////////////////////  CONSTANTS  /////////////////////////////////

// Record content types
type ContentType uint8

const (
	HandshakeContentType ContentType = 22
)

// Handshake record sub-types
type HandshakeType uint8

const (
	// Handshake types
	HelloRequestHandshakeType       HandshakeType = 0
	ClientHelloHandshakeType        HandshakeType = 1
	ServerHelloHandshakeType        HandshakeType = 2
	HelloVerifyRequestHandshakeType HandshakeType = 3
	CertificateHandshakeType        HandshakeType = 11
	ServerKeyExchangeHandshakeType  HandshakeType = 12
	CertificateRequestHandshakeType HandshakeType = 13
	ServerHelloDoneHandshakeType    HandshakeType = 14
	CertificateVerifyHandshakeType  HandshakeType = 15
	ClientKeyExchangeHandshakeType  HandshakeType = 16
	FinishedHandshakeType           HandshakeType = 20
)

// Extension types
type ExtensionType uint16

const (
	RenegotiationInfo    ExtensionType = 65281
	ExtendedMasterSecret ExtensionType = 23
	SessionTicketTLS     ExtensionType = 35
	SignatureAlgorithms  ExtensionType = 13
	UseSRTP              ExtensionType = 14
	ECPointFormats       ExtensionType = 11
	SupportedGroups      ExtensionType = 10
)

// Cipher suites
var TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA = cipherSuite{0xC0, 0x09}

// Signature hash algorithms
const (
	SHA1   = 0x02
	SHA256 = 0x04
	SHA384 = 0x05
	SHA512 = 0x06
)
const (
	RSA   = 0x01
	ECDSA = 0x03
)

// Protection profiles
var SRTP_AES128_CM_HMAC_SHA1_80 = protectionProfile{0x00, 0x01}

///////////////////////////////////  TYPES  ///////////////////////////////////

// DTLS packet
type packet struct {
	records []record
}

// DTLS record
type record struct {
	contentType    ContentType
	version        uint16
	epoch          uint16
	sequenceNumber uint64
	length         uint16
	fragment       interface{}
}

// DTLS handshake record
type handshake struct {
	messageType     HandshakeType
	length          uint32
	messageSequence uint16
	fragmentOffset  uint32
	fragmentLength  uint32
	body            interface{}
	extensions      []extension
}

// Random type
type random struct {
	time  time.Time
	bytes [28]byte
}

// Handshake record: ClientHello
type cipherSuite [2]uint8
type compressionMethod uint8
type clientHello struct {
	version uint16
	random
	sessionID          []byte
	cookie             []byte
	cipherSuites       []cipherSuite
	compressionMethods []compressionMethod
}

// DTLS handshake record extension
type extension struct {
	extensionType ExtensionType
	length        uint16
	data          interface{}
}

type protectionProfile [2]uint8
type useSRTP struct {
	protectionProfiles []protectionProfile
	mki                []byte
}

type signatureAlgorithm struct {
	hash   uint8
	cipher uint8
}
type signatureAlgorithms []signatureAlgorithm

type ecPointFormat uint16
type ecPointFormats []ecPointFormat

type supportedGroup uint16
type supportedGroups []supportedGroup

////////////////////////////////  SERIALIZERS  ////////////////////////////////

func (p *packet) marshal() []byte {
	b := []byte{}

	for _, r := range p.records {
		b = append(b, r.marshal()...)
	}

	return b
}

func (r *record) marshal() []byte {
	var frag []byte

	b := make([]byte, 13, 13)

	// Write header
	b[0] = byte(r.contentType)
	binary.BigEndian.PutUint64(b[3:11], r.sequenceNumber)
	binary.BigEndian.PutUint16(b[1:3], r.version)
	binary.BigEndian.PutUint16(b[3:5], r.epoch)

	// Marshal fragment based on type
	switch r.fragment.(type) {
	case handshake:
		ch := r.fragment.(handshake)
		frag = ch.marshal()
	}

	// Write length of fragment
	r.length = uint16(len(frag))
	binary.BigEndian.PutUint16(b[11:13], r.length)

	// Write marshaled fragment
	b = append(b, frag...)

	return b
}

func (r *random) marshal() []byte {
	b := make([]byte, 32, 32)

	binary.BigEndian.PutUint32(b[0:4], uint32(r.time.Unix()))
	copy(b[4:32], r.bytes[:])

	return b
}

func (h *handshake) marshal() []byte {
	var frag []byte

	b := make([]byte, 12, 12)

	// Marshal fragment based on type
	switch h.body.(type) {
	case clientHello:
		ch := h.body.(clientHello)
		frag = ch.marshal()
	}

	// Marshal extensions
	extensions := make([]byte, 2, 2)
	for _, e := range h.extensions {
		extensions = append(extensions, e.marshal()...)
	}
	binary.BigEndian.PutUint16(extensions[0:2], uint16(len(extensions)-2))

	// Write header
	h.length = uint32(len(frag) + len(extensions))
	binary.BigEndian.PutUint32(b[8:12], h.length) // TODO fix fragment len
	binary.BigEndian.PutUint32(b[5:9], 0)         // TODO fix fragment offset
	binary.BigEndian.PutUint16(b[3:5], h.messageSequence)
	binary.BigEndian.PutUint32(b[0:4], h.length)
	b[0] = byte(h.messageType)

	// Write marshaled fragment
	b = append(b, frag...)

	// Write extensions
	b = append(b, extensions...)

	return b
}

func (h *clientHello) marshal() []byte {
	offset := 0

	slen := len(h.sessionID)
	clen := len(h.cookie)
	nCipherSuites := len(h.cipherSuites)
	nCompressionMethods := len(h.compressionMethods)

	// Compute variable length
	n := slen + clen + 2*nCipherSuites + nCompressionMethods

	b := make([]byte, 39+n, 39+n)

	// Client protocol version
	binary.BigEndian.PutUint16(b[0:2], h.version)

	// Random
	copy(b[2:34], h.random.marshal())

	// Session ID
	b[34] = uint8(slen)
	copy(b[35:], h.sessionID)
	offset += slen

	// Cookie
	b[35+offset] = uint8(clen)
	copy(b[36+offset:], h.cookie)
	offset += clen

	// Cipher suites
	binary.BigEndian.PutUint16(b[36+offset:], uint16(2*nCipherSuites))
	for _, cs := range h.cipherSuites {
		copy(b[38+offset:], cs[:])
		offset += int(unsafe.Sizeof(cs))
	}

	// Compression methods
	b[38+offset] = uint8(nCompressionMethods)
	for i, cm := range h.compressionMethods {
		b[39+offset+i] = uint8(cm)
	}

	return b
}

func (e *extension) marshal() []byte {
	var body []byte

	b := make([]byte, 4, 4)

	// Marshal fragment based on type
	switch e.data.(type) {
	case useSRTP:
		data := e.data.(useSRTP)
		body = make([]byte, 2, 2)
		for _, pp := range data.protectionProfiles {
			body = append(body, pp[0])
			body = append(body, pp[1])
		}
		binary.BigEndian.PutUint16(body[0:2], uint16(len(body[2:])))
		body = append(body, uint8(len(data.mki)))
		body = append(body, data.mki...)
	case []signatureAlgorithm:
		body = make([]byte, 2, 2)
		algos := e.data.([]signatureAlgorithm)
		for _, algo := range algos {
			body = append(body, []byte{algo.hash, algo.cipher}...)
		}
		binary.BigEndian.PutUint16(body[0:2], uint16(len(body[2:])))
		log.Println("sa:", body)
	case []supportedGroup:
		// TODO Single P-256 elliptic curve hardcoded
		body = []byte{0x00, 0x02, 0x00, 0x17}
	}

	binary.BigEndian.PutUint16(b[0:2], uint16(e.extensionType))
	binary.BigEndian.PutUint16(b[2:4], uint16(len(body)))

	b = append(b, body...)

	return b
}

///////////////////////////////  DESERIALIZERS  //////////////////////////////

///////////////////////////////  CONSTRUCTORS  ///////////////////////////////

func newRandom() random {
	r := random{
		time: time.Now(),
	}

	rand.Read(r.bytes[:])

	return r
}

func newClientHello() *record {
	r := &record{
		contentType:    HandshakeContentType,
		version:        0xfefd,
		epoch:          0,
		sequenceNumber: 0,
		fragment: handshake{
			messageType:     ClientHelloHandshakeType,
			messageSequence: 0,
			body: clientHello{
				version:   0xfefd,
				random:    newRandom(),
				sessionID: nil,
				cipherSuites: []cipherSuite{
					TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				},
				compressionMethods: []compressionMethod{
					0,
				},
			},
			extensions: []extension{
				extension{
					extensionType: UseSRTP,
					data: useSRTP{
						protectionProfiles: []protectionProfile{
							SRTP_AES128_CM_HMAC_SHA1_80,
						},
						mki: nil,
					},
				},
				extension{
					extensionType: SignatureAlgorithms,
					data: []signatureAlgorithm{
						signatureAlgorithm{SHA256, ECDSA},
						signatureAlgorithm{SHA1, RSA},
					},
				},
				extension{
					extensionType: SupportedGroups,
					data: []supportedGroup{
						0,
					},
				},
			},
		},
	}

	return r
}

func sendClientHello(w io.Writer) {
	handshake := newClientHello()

	body := handshake.marshal()

	w.Write(body)
}

type DTLSConn struct {
	conn net.Conn
}

func DialDTLS(conn net.Conn) (DTLSConn, error) {
	sendClientHello(conn)

	return DTLSConn{conn}, nil
}
