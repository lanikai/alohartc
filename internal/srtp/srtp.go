package srtp

import (
	"crypto/cipher"
	"encoding/binary"
	"net"
)

// encrypt a SRTP packet in place
func (c *Context) encrypt(m *rtpMsg) bool {
	s := c.getSSRCState(m.ssrc)

	c.updateRolloverCount(m.sequenceNumber, s)

	stream := cipher.NewCTR(c.srtpBlock, c.generateCounter(m.sequenceNumber, s.rolloverCounter, s.ssrc, c.srtpSessionSalt))
	stream.XORKeyStream(m.payload, m.payload)

	fullPkt := m.marshal()

	fullPkt = append(fullPkt, make([]byte, 4)...)
	binary.BigEndian.PutUint32(fullPkt[len(fullPkt)-4:], s.rolloverCounter)

	authTag, err := c.generateAuthTag(fullPkt, c.srtpSessionAuthTag)
	if err != nil {
		return false
	}

	m.payload = append(m.payload, authTag...)
	return true
}

// https://tools.ietf.org/html/rfc3550#appendix-A.1
func (c *Context) updateRolloverCount(sequenceNumber uint16, s *ssrcState) {
	if !s.rolloverHasProcessed {
		s.rolloverHasProcessed = true
	} else if sequenceNumber == 0 { // We exactly hit the rollover count

		// Only update rolloverCounter if lastSequenceNumber is greater then maxROCDisorder
		// otherwise we already incremented for disorder
		if s.lastSequenceNumber > maxROCDisorder {
			s.rolloverCounter++
		}
	} else if s.lastSequenceNumber < maxROCDisorder && sequenceNumber > (maxSequenceNumber-maxROCDisorder) {
		// Our last sequence number incremented because we crossed 0, but then our current number was within maxROCDisorder of the max
		// So we fell behind, drop to account for jitter
		s.rolloverCounter--
	} else if sequenceNumber < maxROCDisorder && s.lastSequenceNumber > (maxSequenceNumber-maxROCDisorder) {
		// our current is within a maxROCDisorder of 0
		// and our last sequence number was a high sequence number, increment to account for jitter
		s.rolloverCounter++
	}
	s.lastSequenceNumber = sequenceNumber
}

func (c *Context) getSSRCState(ssrc uint32) *ssrcState {
	s, ok := c.ssrcStates[ssrc]
	if ok {
		return s
	}

	s = &ssrcState{ssrc: ssrc}
	c.ssrcStates[ssrc] = s
	return s
}

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
		conn: conn,
		typ:  dynamicType, // must match SDP answer (hard-coded for now)
		ssrc: 2541098696,  // must match SDP answer (hard-coded for now)
		seq:  5984,
		time: 3309803758,

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

func (c *Conn) Send(b []byte) error {
	maxSize := 1280

	var err error
	if len(b) < maxSize {
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
		for i := 1; i < len(b); i += maxSize {
			tail := i + maxSize
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
