package dtls

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"strings"
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
var DTLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA = dtlsCipherSuite{0xC0, 0x09}

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

// Handshake certificate
type handshakeCertificate struct {
	certificates	[]x509.Certificate
}

// Handshake server key exchange
type serverKeyExchange struct {
	curveType	uint8
	namedCurve	uint16
	publicKey	[]byte
	signatureAlgorithm
	signature	[]byte
}

// Random type
type random struct {
	time  time.Time
	bytes [28]byte
}

// Handshake client hello
type dtlsCipherSuite [2]uint8
type compressionMethod uint8
type clientHello struct {
	version uint16
	random
	sessionID          []byte
	cookie             []byte
	cipherSuites       []dtlsCipherSuite
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

func (hc *handshakeCertificate) unmarshal(b []byte) error {
	// Verify have at least 3 bytes. First 3 bytes contain length.
	if len(b) < 3 {
		return errors.New("Malformed handshake certificate. Too short.")
	}

	// Read length and verify agrees with number of bytes in slice.
	totlen := int(binary.BigEndian.Uint32(append([]byte{0}, b[0:3]...)))
	if 3 + totlen != len(b) {
		return errors.New("Malformed handshake certificate. Incorrect length.")
	}

	// Parse certificates
	offset := 3
	for offset < len(b) {
		certlen := int(binary.BigEndian.Uint32(append(
			[]byte{0},
			b[offset+0:offset+3]...
		)))
		cert, err := x509.ParseCertificate(b[offset+3:offset+3+certlen])
		if err != nil {
			return err
		}
		hc.certificates = append(hc.certificates, *cert)
		offset += 3 + certlen
	}

	return nil
}

func (ske *serverKeyExchange) unmarshal(b []byte) error {
	if len(b) < 4 {
		return errors.New("Malformed handshake server key exchange. Too short.")
	}

	// Read elliptic curve parameters
	ske.curveType = b[0]
	ske.namedCurve = binary.BigEndian.Uint16(b[1:3])

	// Read public key
	pubkeyLength := int(b[3])
	ske.publicKey = b[4:4+pubkeyLength]

	// Read signature algorithm
	ske.signatureAlgorithm = signatureAlgorithm{
		hash: b[4+pubkeyLength],
		cipher: b[5+pubkeyLength],
	}

	// Read signature
	sigLength := int(binary.BigEndian.Uint16(b[6+pubkeyLength:8+pubkeyLength]))
	ske.signature = b[8+pubkeyLength:8+pubkeyLength+sigLength]

	return nil
}

func (h *handshake) unmarshal(b []byte) error {
	if len(b) < 12 {
		return errors.New("Malformed handshake. Too short.")
	}

	length := int(binary.BigEndian.Uint32(b[0:4]) & 0x00ffffff)
	if 12 + length != len(b) {
		return errors.New("Malformed handshake. Incorrect length.")
	}

	h.messageType = HandshakeType(b[0])

	switch h.messageType {
	case ServerHelloHandshakeType:
		log.Println("Server Hello")
	case ServerHelloDoneHandshakeType:
		log.Println("Server Hello Done")
	case CertificateHandshakeType:
		log.Println("Certificate Handshake")
		hc := handshakeCertificate{}
		if err := hc.unmarshal(b[12:]); err != nil {
			return err
		}
		h.body = hc
	case ServerKeyExchangeHandshakeType:
		log.Println("Server Key Exchange")
		ske := serverKeyExchange{}
		if err := ske.unmarshal(b[12:]); err != nil {
			return err
		}
		h.body = ske
	case CertificateRequestHandshakeType:
		log.Println("Certificate Request")
	}

	h.length = uint32(length)
	h.messageSequence = binary.BigEndian.Uint16(b[4:6])
	h.fragmentOffset = binary.BigEndian.Uint32(b[5:9]) & 0x00ffffff
	h.fragmentLength = binary.BigEndian.Uint32(b[8:12]) & 0x00ffffff

	return nil
}

func (r *record) unmarshal(b []byte) error {
	if len(b) < 13 {
		return errors.New("Malformed record. Too short.")
	}

	if 13 + binary.BigEndian.Uint16(b[11:13]) != uint16(len(b)) {
		return errors.New("Malformed record. Incorrect length.")
	}

	r.contentType = ContentType(b[0])

	switch r.contentType {
	case HandshakeContentType:
		hs := handshake{}
		if err := hs.unmarshal(b[13:]); err != nil {
			return err
		}
		r.fragment = hs
	}

	r.version = binary.BigEndian.Uint16(b[1:3])
	r.epoch = binary.BigEndian.Uint16(b[3:5])
	r.sequenceNumber = 0x0000ffffffffffff & binary.BigEndian.Uint64(b[3:11])
	r.length = uint16(len(b) - 13)

	return nil
}

func (p *packet) unmarshal(b []byte) error {
	offset := uint16(0)

	for offset < uint16(len(b)) {
		r := record{}

		size := 13 + binary.BigEndian.Uint16(b[offset+11:offset+13])
		if (offset + size) > uint16(len(b)) {
			return errors.New("Malformed packet. Invalid length.")
		}
		if err := r.unmarshal(b[offset:offset + size]); err != nil {
			return err
		}

		p.records = append(p.records, r)

		offset += size
	}

	return nil
}


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
				cipherSuites: []dtlsCipherSuite{
					DTLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
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


// Client returns a new TLS client side connection
// using conn as the underlying transport.
// The config cannot be nil: users must set either ServerName or
// InsecureSkipVerify in the config.
func Client(conn net.Conn, config *Config) *Conn {
	return &Conn{conn: conn, config: config, isClient: true}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "tls: DialWithDialer timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func DialWithConnection(rawConn net.Conn) (*Conn, error) {
//	sendClientHello(conn)

	var err error

	// We want the Timeout and Deadline values from dialer to cover the
	// whole process: TCP connection and TLS handshake. This means that we
	// also need to start our own timers now.
	timeout := 60 * time.Second

//	if !dialer.Deadline.IsZero() {
//		deadlineTimeout := time.Until(dialer.Deadline)
//		if timeout == 0 || deadlineTimeout < timeout {
//			timeout = deadlineTimeout
//		}
//	}

	var errChannel chan error

	if timeout != 0 {
		errChannel = make(chan error, 2)
		time.AfterFunc(timeout, func() {
			errChannel <- timeoutError{}
		})
	}

//	rawConn, err := dialer.Dial(network, addr)
//	if err != nil {
//		return nil, err
//	}

	addr := rawConn.LocalAddr().String()
	var config *Config

	colonPos := strings.LastIndex(addr, ":")
	if colonPos == -1 {
		colonPos = len(addr)
	}
	hostname := addr[:colonPos]

	if config == nil {
		config = defaultConfig()
	}
	// If no ServerName is set, infer the ServerName
	// from the hostname we're connecting to.
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument or default.
		c := config.Clone()
		c.ServerName = hostname
		config = c
	}

	conn := Client(rawConn, config)

	if timeout == 0 {
		err = conn.Handshake()
	} else {
		go func() {
			errChannel <- conn.Handshake()
		}()

		err = <-errChannel
	}

	if err != nil {
		rawConn.Close()
		return nil, err
	}

	return conn, nil
}
