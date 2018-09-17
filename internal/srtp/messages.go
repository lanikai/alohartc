package srtp

type rtpMsg struct {
	ptyp      uint8
	timestamp uint32
	marker    bool
	csrc      []uint32 // contributing source identifiers
	ssrc      uint32   // synchronization source identifier
	seq       uint16   // sequence number
	payload   []byte
}

func (m *rtpMsg) marshal() []byte {
	length := 4*len(m.csrc) + len(m.payload)
	b := make([]byte, 12+length)
	b[0] = 2 << 6

	// csrc count
	b[0] |= byte(len(m.csrc)) & 0xF

	// payload type
	b[1] = (m.ptyp & 0x7F)

	// marker bit
	if m.marker {
		b[1] |= 0x80
	}

	// sequence number
	b[2] = byte(m.seq >> 8)
	b[3] = byte(m.seq)

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
