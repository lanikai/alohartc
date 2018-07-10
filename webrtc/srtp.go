package webrtc

import (
	"encoding/binary"
	"time"
)

type SRTP struct {
	Marker bool

	PayloadType uint8

	// Sequence number
	SequenceNumber uint16

	// Synchronization source identifier
	SSRC uint32

	// Contribution source identifiers
	CSRC []uint32

	// Optional extension
	Extension []byte

	Data []byte
}

const (
	secondsFrom1900To1970 uint32 = 2208988800
)

// MarshalBinary encodes packet struct into byte array
func (p *SRTP) MarshalBinary() ([]byte, error) {
	// Packet size
	n := 12 + 4 * len(p.CSRC) + len(p.Data)

	// Create buffer
	b := make([]byte, n, n)

	// Set version
	b[0] = 0x80

	// Set number of contribution source identifiers
	b[0] |= uint8(len(p.CSRC)) & 0xF

	// Set marker
	if p.Marker {
		b[1] |= 0x80
	}

	// Set payload type
	b[1] |= p.PayloadType & 0x7F

	// Set sequence number
	binary.BigEndian.PutUint16(b[2:4], p.SequenceNumber)

	// Timestamp (seconds since 1900)
	binary.BigEndian.PutUint32(b[4:8],
		secondsFrom1900To1970 + uint32(time.Now().Unix()),
	)

	// Synchronization source identifier
	binary.BigEndian.PutUint32(b[8:12], p.SSRC)

	// Contribution source identifier(s)
	for i, csrc := range p.CSRC {
		binary.BigEndian.PutUint32(b[12 + 4 * i : 16 + 4 * i], csrc)
	}

	// Copy payload
	copy(b[12 + 4 * len(p.CSRC) :], p.Data)

	return b, nil
}

// UnmarshalBinary decodes byte array into packet struct
func (p *SRTP) UnmarshalBinary(b []byte) {
}
