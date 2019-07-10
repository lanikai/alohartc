// Copyright 2019 Lanikai Labs. All rights reserved.

package srtp

import (
	"encoding/binary"
)

type rtpMsg struct {
	marker         bool
	payloadType    uint8
	sequenceNumber uint16
	timestamp      uint32
	ssrc           uint32
	csrc           []uint32
	payload        []byte
}

// marshal RTP message into a byte slice
func (m *rtpMsg) marshal() []byte {
	length := 4*len(m.csrc) + len(m.payload)
	b := make([]byte, 12+length)
	b[0] = 2 << 6

	// csrc count
	b[0] |= byte(len(m.csrc)) & 0xF

	// payload type
	b[1] = (m.payloadType & 0x7F)

	// marker bit
	if m.marker {
		b[1] |= 0x80
	}

	// sequence number
	b[2] = byte(m.sequenceNumber >> 8)
	b[3] = byte(m.sequenceNumber)

	// timestamp
	b[4] = byte(m.timestamp >> 24)
	b[5] = byte(m.timestamp >> 16)
	b[6] = byte(m.timestamp >> 8)
	b[7] = byte(m.timestamp)

	// synchronization source identifier
	b[8] = byte(m.ssrc >> 24)
	b[9] = byte(m.ssrc >> 16)
	b[10] = byte(m.ssrc >> 8)
	b[11] = byte(m.ssrc)

	// contributing source identifiers
	for i, csrc := range m.csrc {
		b[12+4*i] = byte(csrc >> 24)
		b[13+4*i] = byte(csrc >> 16)
		b[14+4*i] = byte(csrc >> 8)
		b[15+4*i] = byte(csrc)
	}

	// payload
	copy(b[12+4*len(m.csrc):], m.payload)

	return b
}

// unmarshal byte slice into RTP message
func (m *rtpMsg) unmarshal(b []byte) error {
	// Verify packet length is greater than RTP header
	if len(b) < 12 {
		return errMalformedPacket
	}

	// Check RTP version
	if 2 != b[0]>>6 {
		return errUnsupportedVersion
	}

	// Get expected number of CSRCs
	csrcCount := int(b[0] & 0x0F)

	// Verify that packet length sufficient for claimed number of CSRCs
	if len(b) < 12+4*csrcCount {
		return errMalformedPacket
	}

	// payload type
	m.payloadType = uint8(b[1] & 0x7F)

	// marker bit
	m.marker = (0x80 == b[1]&0x80)

	// sequence number
	m.sequenceNumber = binary.BigEndian.Uint16(b[2:])

	// timestamp
	m.timestamp = binary.BigEndian.Uint32(b[4:])

	// synchronization source identifier
	m.ssrc = binary.BigEndian.Uint32(b[8:])

	// contributing source identifiers
	for i := 0; i < csrcCount; i++ {
		m.csrc = append(m.csrc, binary.BigEndian.Uint32(b[12+4*i:]))
	}

	// copy payload
	m.payload = make([]byte, len(b)-12-4*csrcCount)
	copy(m.payload, b[12+4*csrcCount:])

	return nil
}
