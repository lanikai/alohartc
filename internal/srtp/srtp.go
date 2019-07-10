package srtp

import (
	"net"
)

const (
	maxPacketSize = 1280
)

type Conn struct {
	conn net.Conn
	ssrc uint32
	seq  uint16
	typ  uint8
	time uint32

	context *Context
}

func NewSession(conn net.Conn, dynamicType uint8, masterKey, masterSalt []byte) (*Conn, error) {
	ctx, err := CreateContext(masterKey, masterSalt)
	if err != nil {
		return nil, err
	}

	return &Conn{
		conn:    conn,
		typ:     dynamicType,
		ssrc:    2541098696, // must match SDP answer (hard-coded for now)
		seq:     5984,
		time:    3309803758,
		context: ctx,
	}, nil
}

// Close the underlying connection.
func (c *Conn) Close() {
	c.conn.Close()
}

func (c *Conn) Stap(b []byte) error {
	m := rtpMsg{
		payloadType:    c.typ,
		timestamp:      c.time,
		marker:         false,
		csrc:           []uint32{},
		ssrc:           c.ssrc,
		sequenceNumber: c.seq,
		payload:        b,
	}
	c.context.encrypt(&m)
	_, err := c.conn.Write(m.marshal())
	c.seq += 1
	return err
}

// Read next frame from connection
func (c *Conn) Read(b []byte) (int, error) {
	var m rtpMsg

	// new packet buffer
	buffer := make([]byte, maxPacketSize)

	// read next packet
	n, err := c.conn.Read(buffer)
	if err != nil {
		return 0, err
	}

	// parse packet
	if err := m.unmarshal(buffer[:n]); err != nil {
		return 0, err
	}

	// decipher
	c.context.decrypt(&m)

	return copy(b, m.payload), nil
}

func (c *Conn) Send(b []byte) error {
	var err error
	if len(b) < maxPacketSize {
		m := rtpMsg{
			payloadType:    c.typ,
			timestamp:      c.time,
			marker:         false,
			csrc:           []uint32{},
			ssrc:           c.ssrc,
			sequenceNumber: c.seq,
			payload:        b,
		}
		c.context.encrypt(&m)
		_, err = c.conn.Write(m.marshal())
		c.seq += 1
	} else {
		indicator := byte((0 & 0x80) | (b[0] & 0x60) | 28)
		start := byte(0x80)
		end := byte(0)
		typ := byte(b[0] & 0x1F)
		mark := false
		for i := 1; i < len(b); i += maxPacketSize {
			tail := i + maxPacketSize
			if tail >= len(b) {
				end = 0x40
				tail = len(b)
			}
			header := byte(start | end | typ)
			data := append([]byte{indicator, header}, b[i:tail]...)

			if end != 0 {
				mark = true
			}

			m := rtpMsg{
				payloadType:    c.typ,
				timestamp:      c.time,
				marker:         mark,
				csrc:           []uint32{},
				ssrc:           c.ssrc,
				sequenceNumber: c.seq,
				payload:        data,
			}
			c.context.encrypt(&m)
			_, err = c.conn.Write(m.marshal())
			if err != nil {
				break
			}

			c.seq += 1

			start = 0
		}
	}

	// TODO This should be replaced with the difference between successive
	// TODO timecodes returned by the v4l2 device (or other future source).
	c.time += 3000

	return err
}
