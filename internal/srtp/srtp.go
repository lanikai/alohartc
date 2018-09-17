package srtp

import (
	"encoding/binary"
	"net"
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
	n := 12 + 4*len(p.CSRC) + len(p.Data)

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
		secondsFrom1900To1970+uint32(time.Now().Unix()),
	)

	// Synchronization source identifier
	binary.BigEndian.PutUint32(b[8:12], p.SSRC)

	// Contribution source identifier(s)
	for i, csrc := range p.CSRC {
		binary.BigEndian.PutUint32(b[12+4*i:16+4*i], csrc)
	}

	// Copy payload
	copy(b[12+4*len(p.CSRC):], p.Data)

	return b, nil
}

// UnmarshalBinary decodes byte array into packet struct
func (p *SRTP) UnmarshalBinary(b []byte) {
}

type Conn struct {
	conn *net.UDPConn
	ssrc uint32
	seq  uint16
	typ  uint8
	time uint32
}

func NewSession(conn *net.UDPConn) (*Conn, error) {
	// TODO Fix hard-coded dynamic RTP type,
	return &Conn{
		conn: conn,
		typ:  100,       // must match SDP answer (hard-coded for now)
		ssrc: 541098696, // must match SDP answer (hard-coded for now)
		seq:  5984,
		time: 3309803758,
	}, nil
}

func (c *Conn) Close() {
}

func (c *Conn) Send(b []byte) {

	if len(b) < 1000 {
		m := rtpMsg{
			c.typ,
			c.time,
			true,
			[]uint32{},
			c.ssrc,
			c.seq,
			b,
		}
		c.conn.Write(m.marshal())
		c.seq += 1
	} else {
		indicator := byte(0x80 | (b[0] & 0x60) | 28)
		start := byte(0x80)
		end := byte(0)
		typ := byte(b[0] & 0x1F)
		for i := 1; i < len(b); i += 1000 {
			tail := i + 1000
			if tail > len(b) {
				end = 0x40
				tail = len(b)
			}
			header := byte(start | end | typ)
			data := append([]byte{indicator, header}, b[i:tail]...)

			m := rtpMsg{
				c.typ,
				c.time,
				true,
				[]uint32{},
				c.ssrc,
				c.seq,
				data,
			}
			c.conn.Write(m.marshal())

			c.seq += 1

			start = 0
		}
	}

	c.time += 3000
	time.Sleep(32 * time.Millisecond)
}
