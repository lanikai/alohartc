package ice

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"strings"
)

// STUN (Sessian Traversal Utilities for NAT)
// RFC 5389 (https://tools.ietf.org/html/rfc5389).

type stunMessage struct {
	// Message length in bytes, NOT including the 20-byte header.
	length uint16

	// Message class, 2 bits.
	class uint16

	// Message method, 12 bits.
	method uint16

	// Globally unique transaction ID, 12 bytes.
	transactionID string

	// Attributes with meaning determined by the class and method.
	attributes []*stunAttribute
}

// Returns (nil, nil) if the data is not a STUN message.
func parseStunMessage(data []byte) (*stunMessage, error) {
	msg := parseStunHeader(data[0:stunHeaderLength])
	if msg == nil {
		return nil, nil
	}

	// Parse attributes.
	b := bytes.NewBuffer(data[stunHeaderLength:])
	for b.Len() > 0 {
		attr, err := parseStunAttribute(b)
		if err != nil {
			return msg, err
		}

		// TODO: check message integrity and fingerprint
		msg.attributes = append(msg.attributes, attr)
	}
	return msg, nil
}

func writeStunMessage(msg *stunMessage, b *bytes.Buffer) {
	writeStunHeader(msg, b)
	for _, attr := range msg.attributes {
		writeStunAttribute(attr, b)
	}
}

func (msg *stunMessage) String() string {
	b := new(strings.Builder)
	switch msg.class {
	case stunRequest:
		b.WriteString("STUN request")
	case stunIndication:
		b.WriteString("STUN indication")
	case stunSuccessResponse:
		b.WriteString("STUN success response")
	case stunErrorResponse:
		b.WriteString("STUN error response")
	}
	if msg.method != stunBindingMethod {
		fmt.Fprintf(b, ", method %x", msg.method)
	}
	fmt.Fprintf(b, ", tid=%s", hex.EncodeToString([]byte(msg.transactionID)))
	for _, attr := range msg.attributes {
		switch attr.Type {
		case stunAttrMappedAddress:
			fmt.Fprintf(b, ", MAPPED-ADDRESS %s", extractAddr(attr, msg.transactionID, false))
		case stunAttrXorMappedAddress:
			fmt.Fprintf(b, ", XOR-MAPPED-ADDRESS %s", extractAddr(attr, msg.transactionID, true))
		case stunAttrUsername:
			fmt.Fprintf(b, ", USERNAME %s", string(attr.Value))
		case stunAttrErrorCode:
			fmt.Fprintf(b, ", ERROR-CODE %s", string(attr.Value))
		case stunAttrUnknownAttributes:
			fmt.Fprintf(b, ", UNKNOWN %s", string(attr.Value))
		case stunAttrUseCandidate:
			fmt.Fprintf(b, ", USE-CANDIDATE")
		case stunAttrIceControlled:
			fmt.Fprintf(b, ", ICE-CONTROLLED")
		case stunAttrIceControlling:
			fmt.Fprintf(b, ", ICE-CONTROLLING")
		case stunAttrPriority:
			fmt.Fprintf(b, ", PRIORITY ?")
		case stunAttrSoftware:
		case stunAttrFingerprint:
		case stunAttrMessageIntegrity:
			// Ignore these
		default:
			fmt.Fprintf(b, ", unknown attribute %x", attr.Type)
		}
	}
	return b.String()
}

// Allowed STUN message classes.
const (
	stunRequest         = 0
	stunIndication      = 1
	stunSuccessResponse = 2
	stunErrorResponse   = 3
)

const stunBindingMethod = 0x1

const stunHeaderLength = 20
const stunMagicCookie = 0x2112A442

// Figure 2: Format of STUN Message Header
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |0 0|     STUN Message Type     |         Message Length        |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                         Magic Cookie                          |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                                                               |
//    |                     Transaction ID (96 bits)                  |
//    |                                                               |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

// Returns nil if the data does not look like a STUN message.
func parseStunHeader(data []byte) *stunMessage {
	if len(data) < stunHeaderLength {
		return nil
	}

	// The top two bits of the message type must be 0.
	messageType := binary.BigEndian.Uint16(data[0:2])
	if messageType>>14 != 0 {
		return nil
	}

	// The length must be a multiple of 4 bytes.
	length := binary.BigEndian.Uint16(data[2:4])
	if length%4 != 0 {
		return nil
	}

	// The magic cookie must be present.
	magicCookie := binary.BigEndian.Uint32(data[4:8])
	if magicCookie != stunMagicCookie {
		return nil
	}

	class, method := decomposeMessageType(messageType)
	msg := &stunMessage{
		length:        length,
		class:         class,
		method:        method,
		transactionID: string(data[8:20]),
	}
	return msg
}

func writeStunHeader(msg *stunMessage, b *bytes.Buffer) {
	messageType := composeMessageType(msg.class, msg.method)
	binary.BigEndian.PutUint16(b.Next(2), messageType)
	binary.BigEndian.PutUint16(b.Next(2), msg.length)
	binary.BigEndian.PutUint32(b.Next(4), stunMagicCookie)
	copy(b.Next(12), msg.transactionID)
}

// Figure 3: Format of STUN Message Type Field
//     0                 1
//     2  3  4 5 6 7 8 9 0 1 2 3 4 5
//    +--+--+-+-+-+-+-+-+-+-+-+-+-+-+
//    |M |M |M|M|M|C|M|M|M|C|M|M|M|M|
//    |11|10|9|8|7|1|6|5|4|0|3|2|1|0|
//    +--+--+-+-+-+-+-+-+-+-+-+-+-+-+
const classMask1 = 0x0100  // 0b0000000100000000
const classMask2 = 0x0010  // 0b0000000000010000
const methodMask1 = 0x3e00 // 0b0011111000000000
const methodMask2 = 0x00e0 // 0b0000000011100000
const methodMask3 = 0x000f // 0b0000000000001111

func composeMessageType(class uint16, method uint16) uint16 {
	t := (class<<7)&classMask1 | (class<<4)&classMask2
	t |= (method<<2)&methodMask1 | (method<<1)&methodMask2 | (method & methodMask3)
	return t
}

func decomposeMessageType(t uint16) (uint16, uint16) {
	class := (t&classMask1)>>7 | (t&classMask2)>>4
	method := (t&methodMask1)>>2 | (t&methodMask2)>>1 | (t & methodMask3)
	return class, method
}

// Figure 4: Format of STUN Attributes
//     0                   1                   2                   3
//     0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |         Type                  |            Length             |
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//    |                         Value (variable)                ....
//    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type stunAttribute struct {
	Type   uint16
	Length uint16
	Value  []byte
}

func parseStunAttribute(b *bytes.Buffer) (*stunAttribute, error) {
	if b.Len() < 4 {
		// TODO: error handling
		return nil, fmt.Errorf("Invalid STUN attribute: %s", b.Bytes())
	}

	typ := binary.BigEndian.Uint16(b.Next(2))
	length := binary.BigEndian.Uint16(b.Next(2))
	if int(length) > b.Len() {
		return nil, fmt.Errorf("Illegal STUN attribute: type=%d, length=%d", typ, length)
	}
	value := make([]byte, length)
	copy(value, b.Next(int(length)))
	b.Next(pad4(length)) // discard bytes until next 4-byte boundary
	return &stunAttribute{typ, length, value}, nil
}

func writeStunAttribute(attr *stunAttribute, b *bytes.Buffer) {
	binary.BigEndian.PutUint16(b.Next(2), attr.Type)
	binary.BigEndian.PutUint16(b.Next(2), attr.Length)
	copy(b.Next(int(attr.Length)), attr.Value)
	copy(b.Next(pad4(attr.Length)), zeros)
}

// Return the total size of the attribute in bytes, including the header and padding.
func (attr *stunAttribute) numBytes() int {
	return 4 + int(attr.Length) + pad4(attr.Length)
}

// Return the number of extra bytes needed to pad the given length to a 4-byte boundary.
// The result will be either 0, 1, 2, or 3.
func pad4(n uint16) int {
	return -int(n) & 3
}

var zeros = make([]byte, 32)

// If transactionID is empty, a random transaction ID will be generated.
func newStunMessage(class uint16, method uint16, transactionID string) *stunMessage {
	if class>>2 != 0 {
		log.Panicf("Invalid STUN message class: %#x", class)
	}
	if method>>12 != 0 {
		log.Panicf("Invalid STUN method: %#x", method)
	}

	if transactionID == "" {
		// Generate a random transaction ID.
		buf := make([]byte, 12)
		rand.Read(buf)
		transactionID = string(buf)
	} else if len(transactionID) != 12 {
		log.Panicf("Invalid transaction ID: %s", transactionID)
	}
	msg := &stunMessage{
		length:        0,
		class:         class,
		method:        method,
		transactionID: transactionID,
	}
	return msg
}

func newStunBindingRequest(transactionID string) *stunMessage {
	return newStunMessage(stunRequest, stunBindingMethod, transactionID)
}

func newStunBindingResponse(transactionID string, raddr net.Addr, password string) *stunMessage {
	msg := newStunMessage(stunSuccessResponse, stunBindingMethod, transactionID)
	msg.setXorMappedAddress(raddr)
	// TODO: Make this an option.
	msg.addAttribute(stunAttrIceControlled, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	msg.addMessageIntegrity(password)
	msg.addFingerprint()
	return msg
}

func newStunBindingIndication() *stunMessage {
	msg := newStunMessage(stunIndication, stunBindingMethod, "")
	msg.addFingerprint()
	return msg
}

func (msg *stunMessage) addAttribute(t uint16, v []byte) *stunAttribute {
	l := uint16(len(v))
	// TODO: fix this mess
	vcopy := make([]byte, l)
	copy(vcopy, v)
	attr := &stunAttribute{t, l, vcopy}
	msg.attributes = append(msg.attributes, attr)
	msg.length += uint16(attr.numBytes())
	return attr
}

func (msg *stunMessage) Bytes() []byte {
	buf := make([]byte, stunHeaderLength+msg.length)
	writeStunMessage(msg, bytes.NewBuffer(buf))
	return buf
}

const (
	stunAttrMappedAddress     = 0x0001
	stunAttrUsername          = 0x0006
	stunAttrMessageIntegrity  = 0x0008
	stunAttrErrorCode         = 0x0009
	stunAttrUnknownAttributes = 0x000A
	stunAttrXorMappedAddress  = 0x0020
	stunAttrPriority          = 0x0024
	stunAttrUseCandidate      = 0x0025
	stunAttrSoftware          = 0x8022
	stunAttrFingerprint       = 0x8028
	stunAttrIceControlled     = 0x8029
	stunAttrIceControlling    = 0x802A
)

const stunMagicCookieBytes = "\x21\x12\xA4\x42"
const stunFingerprintXorBytes = "\x53\x54\x55\x4e"

func (msg *stunMessage) getMappedAddress() *net.UDPAddr {
	for _, attr := range msg.attributes {
		if attr.Type == stunAttrMappedAddress {
			return extractAddr(attr, msg.transactionID, false)
		}
		if attr.Type == stunAttrXorMappedAddress {
			return extractAddr(attr, msg.transactionID, true)
		}
	}
	return nil
}

func extractAddr(attr *stunAttribute, transactionID string, doXor bool) *net.UDPAddr {
	addr := new(net.UDPAddr)
	addr.Port = int(binary.BigEndian.Uint16(attr.Value[2:4]))

	family := attr.Value[1]
	switch family {
	case 0x01: // IPv4
		addr.IP = make([]byte, 4)
		copy(addr.IP, attr.Value[4:8])
	case 0x02: // IPv6
		addr.IP = make([]byte, 16)
		copy(addr.IP, attr.Value[4:20])
	default:
		log.Panicf("Invalid mapped address family: %#x", family)
	}

	if doXor {
		addr.Port ^= stunMagicCookie >> 16
		xorBytes(addr.IP[0:4], stunMagicCookieBytes)
		xorBytes(addr.IP[4:], transactionID)
	}
	return addr
}

func (msg *stunMessage) setXorMappedAddress(addr net.Addr) {
	var ip net.IP
	var port int
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	case *net.TCPAddr:
		ip = a.IP
		port = a.Port
	}

	var value []byte
	if ip.To4() != nil {
		// IPv4
		value = make([]byte, 8)
		value[1] = 0x01
		copy(value[4:8], ip.To4())
	} else {
		// IPv6
		value = make([]byte, 20)
		value[1] = 0x02
		copy(value[4:20], ip.To16())
	}
	binary.BigEndian.PutUint16(value[2:4], uint16(port))

	xorBytes(value[2:4], stunMagicCookieBytes[0:2])
	xorBytes(value[4:8], stunMagicCookieBytes)
	xorBytes(value[8:], msg.transactionID)
	msg.addAttribute(stunAttrXorMappedAddress, value)
}

func xorBytes(dest []byte, xor string) {
	for i := range dest {
		dest[i] ^= xor[i]
	}
}

// RFC 5389 Section 15.4. MESSAGE-INTEGRITY
func (msg *stunMessage) addMessageIntegrity(password string) {
	// Use the password to make a new HMAC hash, which has sig.Size() == 20
	sig := hmac.New(sha1.New, []byte(password))

	// Add a dummy MESSAGE-INTEGRITY attribute, such that it is included in msg.length.
	attr := msg.addAttribute(stunAttrMessageIntegrity, zeros[0:20])

	// Compute hash of the message contents up to *just before* the MESSAGE-INTEGRITY.
	b := msg.Bytes()
	beforeMessageIntegrity := len(b) - attr.numBytes()
	sig.Write(b[0:beforeMessageIntegrity])

	copy(attr.Value, sig.Sum(nil))
}

// RFC 5389 Section 15.5. FINGERPRINT
func (msg *stunMessage) addFingerprint() {
	// Add a dummy FINGERPRINT attribute, such that it is included in msg.length.
	attr := msg.addAttribute(stunAttrFingerprint, zeros[0:4])

	// Compute the CRC32 of the message up to *just before* the FINGERPRINT.
	b := msg.Bytes()
	beforeFingerprint := len(b) - attr.numBytes()
	var crc uint32 = crc32.ChecksumIEEE(b[0:beforeFingerprint])

	binary.BigEndian.PutUint32(attr.Value, crc^0x5354554e)
}

func (msg *stunMessage) addPriority(p uint32) {
	v := make([]byte, 4)
	binary.BigEndian.PutUint32(v, p)
	msg.addAttribute(stunAttrPriority, v)
}

func (msg *stunMessage) getPriority() uint32 {
	for _, attr := range msg.attributes {
		if attr.Type == stunAttrPriority {
			return binary.BigEndian.Uint32(attr.Value)
		}
	}
	return 0
}

// Check if the STUN message has a USE-CANDIDATE attribute.
func (msg *stunMessage) hasUseCandidate() bool {
	for _, attr := range msg.attributes {
		if attr.Type == stunAttrUseCandidate {
			return true
		}
	}
	return false
}
