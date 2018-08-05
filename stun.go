package webrtc

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"strings"
	"time"
)

func (pc *PeerConnection) stunBinding(candidate string, key string) error {
	log.Println("ICE candidate:", candidate)
	fields := strings.Fields(candidate)

	// Skip non-UDP
	if protocol := strings.ToLower(fields[2]); protocol != "udp" {
		log.Println("Not UDP, skipping")
		return nil
	}

	//ip, port, ufrag := fields[4], fields[5], fields[11]
	ip, port := fields[4], fields[5]
	log.Println(ip, port)

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
	copy(b[24:28], ufrag)

	originalLength := b[3]
	b[3] = 0x44

	sig := hmac.New(sha1.New, []byte(key))
	sig.Write(b[0:64])

	mig := []byte(sig.Sum(nil))

	b[3] = originalLength

	copy(b[68:88], mig)

	crc32q := crc32.MakeTable(crc32.IEEE)
	crc := crc32.Checksum(b[0:88], crc32q)
	crc = crc ^ 0x5354554e
	binary.BigEndian.PutUint32(b[92:96], crc)

	// Send STUN binding request to caller
	if n, err := pc.conn.Write(b); err != nil {
		log.Println(n, err)
	}

	// Await STUN binding response from caller
	pkt := make([]byte, 1500)
	pc.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := pc.conn.Read(pkt)
	if err != nil {
		log.Fatal(n, err)
	}

/*
	clientHello := []byte{
	0x16, 0xfe, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x68, 0x01, 0x00, 0x00,
	0x5c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x5c, 0xfe, 0xfd, 0x26, 0x44, 0x54, 0x35, 0x89,
	0xab, 0x2c, 0x3e, 0x44, 0xa7, 0x41, 0x27, 0x9a, 0x19, 0x23, 0x7f, 0xf2, 0xe2, 0xcc, 0xfd, 0x34,
	0x9e, 0x14, 0x15, 0x00, 0x8b, 0x27, 0x62, 0xc4, 0x6a, 0xe1, 0x61, 0x00, 0x00, 0x00, 0x02,
	                                          0xc0, 0x09,
	                                          0x01, 0x00, 0x00, 0x30, 0xff, 0x01, 0x00, 0x01, 0x00,
	0x00, 0x17, 0x00, 0x00, 0x00, 0x23, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x06, 0x00, 0x04, 0x04, 0x03,
	                                                                                    0x02, 0x01,
	0x00, 0x0e, 0x00, 0x05, 0x00, 0x02, 0x00, 0x01, 0x00, 0x00, 0x0b, 0x00, 0x02, 0x01, 0x00, 0x00,
	0x0a, 0x00, 0x06, 0x00, 0x04, 0x00, 0x1d, 0x00, 0x17,
	}
	conn.Write(clientHello)
*/

	// Await STUN binding request from caller
	pc.conn.SetReadDeadline(time.Now().Add(time.Second))
	n, addr, err := pc.conn.ReadFrom(pkt)
	if err != nil {
		log.Println(err)
		return err
	}

	// Send STUN binding response to caller
	response := []byte{
		0x01, 0x01, 0x00, 0x2c, 0x21, 0x12, 0xa4, 0x42,
		0x36, 0x30, 0x69, 0x43, 0x58, 0x74, 0x51, 0x44,
		0x58, 0x54, 0x58, 0x45, 0x00, 0x20, 0x00, 0x08,
		0x00, 0x01, 0xff, 0x83, 0x2b, 0xa6, 0x2c, 0x6c,
		0x00, 0x08, 0x00, 0x14, 0x0a, 0x70, 0x20, 0x84,
		0x97, 0xc4, 0x93, 0x28, 0x1f, 0xd7, 0x59, 0xae,
		0x52, 0xfe, 0xa5, 0xe2, 0xca, 0x5b, 0x0a, 0xb6,
		0x80, 0x28, 0x00, 0x04, 0x7f, 0xfb, 0xb0, 0x17,
	}
	// Must use same message transaction ID
	copy(response[8:20], pkt[8:20])
	// Set IP and port
	binary.BigEndian.PutUint16(response[26:28], uint16(addr.(*net.UDPAddr).Port) ^ 0x2112)
	rip := addr.(*net.UDPAddr).IP
	rip[0] ^= 0x21
	rip[1] ^= 0x12
	rip[2] ^= 0xa4
	rip[3] ^= 0x42
	copy(response[28:32], rip)

	// Compute message integrity
	originalLength = response[3]
	response[3] = 0x24

	sig = hmac.New(sha1.New, []byte("auh7I7RsuhlZQgS2XYLStR05"))
	sig.Write(response[0:32])

	mig = []byte(sig.Sum(nil))

	response[3] = originalLength

	copy(response[36:56], mig)

	crc32q = crc32.MakeTable(crc32.IEEE)
	crc = crc32.Checksum(response[0:56], crc32q)
	crc = crc ^ 0x5354554e
	binary.BigEndian.PutUint32(response[60:64], crc)

	time.Sleep(10 * time.Millisecond)
	pc.conn.Write(response)

	return nil
}

// Implementation of STUN (Sessian Traversal Utilities for NAT) following RFC5389
// (https://tools.ietf.org/html/rfc5389).

type stunMessage struct {
	// Message length in bytes, NOT including the 20-byte header.
	length uint16

	// Message class, 2 bits.
	class uint16

	// Message method, 12 bits.
	method uint16

	// Globally unique transaction ID, 12 bytes.
	transactionID []byte

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
	b.WriteString(fmt.Sprintf("length: %d\n", msg.length))
	b.WriteString(fmt.Sprintf("class: %d\n", msg.class))
	b.WriteString(fmt.Sprintf("method: %d\n", msg.method))
	b.WriteString(fmt.Sprintf("transactionID: %v\n", msg.transactionID))
	for _, attr := range msg.attributes {
		b.WriteString(fmt.Sprintf("attribute: %+v\n", *attr))
	}
	return b.String()
}


// Allowed STUN message classes.
const (
	stunRequest			= 0
	stunIndication		= 1
	stunSuccessResponse = 2
	stunErrorResponse	= 3
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
	if messageType >> 14 != 0 {
		return nil
	}

	// The length must be a multiple of 4 bytes.
	length := binary.BigEndian.Uint16(data[2:4])
	if length % 4 != 0 {
		return nil
	}

	// The magic cookie must be present.
	magicCookie := binary.BigEndian.Uint32(data[4:8])
	if magicCookie != stunMagicCookie {
		return nil
	}

	class, method := decomposeMessageType(messageType)
	msg := &stunMessage{
		length: length,
		class: class,
		method: method,
		transactionID: make([]byte, 12),
	}
	copy(msg.transactionID, data[8:20])
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
const classMask1  = 0x0100  // 0b0000000100000000
const classMask2  = 0x0010  // 0b0000000000010000
const methodMask1 = 0x3e00  // 0b0011111000000000
const methodMask2 = 0x00e0  // 0b0000000011100000
const methodMask3 = 0x000f  // 0b0000000000001111

func composeMessageType(class uint16, method uint16) uint16 {
	t := (class << 7) & classMask1 | (class << 4) & classMask2
	t |= (method << 2) & methodMask1 | (method << 1) & methodMask2 | (method & methodMask3)
	return t
}

func decomposeMessageType(t uint16) (uint16, uint16) {
	class := (t & classMask1) >> 7 | (t & classMask2) >> 4
	method := (t & methodMask1) >> 2 | (t & methodMask2) >> 1 | (t & methodMask3)
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
	Type uint16
	Length uint16
	Value []byte
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
	b.Next(pad4(length))  // discard bytes until next 4-byte boundary
	return &stunAttribute{ typ, length, value }, nil
}

func writeStunAttribute(attr *stunAttribute, b *bytes.Buffer) {
	binary.BigEndian.PutUint16(b.Next(2), attr.Type)
	binary.BigEndian.PutUint16(b.Next(2), attr.Length)
	copy(b.Next(int(attr.Length)), attr.Value)
	copy(b.Next(pad4(attr.Length)), zeros)
}

// Return the number of extra bytes needed to pad the given length to a 4-byte boundary.
// The result will be either 0, 1, 2, or 3.
func pad4(n uint16) int {
	return -int(n) & 3
}

var zeros = []byte{ 0, 0, 0, 0 }

// If transactionID is nil, a random transaction ID will be generated.
func newStunMessage(class uint16, method uint16, transactionID []byte) (*stunMessage, error) {
	if class >> 2 != 0 {
		return nil, fmt.Errorf("Invalid STUN message class: %#x", class)
	}
	if method >> 12 != 0 {
		return nil, fmt.Errorf("Invalid STUN method: %#x", method)
	}

	msg := &stunMessage{
		length: 0,
		class: class,
		method: method,
		transactionID: make([]byte, 12),
	}
	if transactionID == nil {
		// Generate a random transaction ID.
		rand.Read(msg.transactionID)
	} else if len(transactionID) != 12 {
		return nil, fmt.Errorf("Invalid transaction ID: %s", transactionID)
	} else {
		copy(msg.transactionID, transactionID)
	}
	return msg, nil
}

func newStunBindingRequest() *stunMessage {
	msg, _ := newStunMessage(stunRequest, stunBindingMethod, nil)
	return msg
}

func (msg *stunMessage) AddAttribute(t uint16, v []byte) {
	l := uint16(len(v))
	msg.attributes = append(msg.attributes, &stunAttribute{t, l, v})
	msg.length += 4 + l + uint16(pad4(l))
}

func (msg *stunMessage) Bytes() []byte {
	buf := make([]byte, stunHeaderLength + msg.length)
	writeStunMessage(msg, bytes.NewBuffer(buf))
	return buf
}


const stunAttrMappedAddress = 0x0001
const stunAttrUsername = 0x0006
const stunAttrMessageIntegrity = 0x0008
const stunAttrErrorCode = 0x0009
const stunAttrUnknownAttributes = 0x000A
const stunAttrXorMappedAddress = 0x0020
const stunAttrSoftware = 0x8022
const stunAttrFingerprint = 0x8028

var stunMagicCookieBytes = []byte{ 0x21, 0x12, 0xA4, 0x42 }

func (msg *stunMessage) getMappedAddress() (*net.UDPAddr, error) {
	if msg.class != stunSuccessResponse {
		return nil, fmt.Errorf("STUN message is not a success response")
	}

	addr := new(net.UDPAddr)
	for _, attr := range msg.attributes {
		if attr.Type == stunAttrXorMappedAddress {
			family := attr.Value[1]
			addr.Port = int(binary.BigEndian.Uint16(attr.Value[2:4]) ^ (stunMagicCookie >> 16))
			switch family {
			case 0x01:
				// IPv4
				addr.IP = attr.Value[4:8]
				xorBytes(addr.IP, stunMagicCookieBytes)
			case 0x02:
				// IPv6
				addr.IP = attr.Value[4:20]
				xorBytes(addr.IP[0:4], stunMagicCookieBytes)
				xorBytes(addr.IP[4:16], msg.transactionID)
			default:
				return nil, fmt.Errorf("Invalid mapped address family: %#x", family)
			}
			return addr, nil
		}
	}

	return nil, fmt.Errorf("STUN message does not have XOR-MAPPED-ADDRESS attribute")
}

func xorBytes(dest []byte, xor []byte) {
	for i := range dest {
		dest[i] ^= xor[i]
	}
}
