package srtp

import (
	"math/rand"
	"net"
)

type AudioConn struct {
	conn net.Conn
	ssrc uint32
	seq  uint16
	typ  uint8
	time uint32

	context *Context
}

func NewAudioSession(conn net.Conn, dynamicType uint8, masterKey, masterSalt []byte) (*AudioConn, error) {
	ctx, err := CreateContext(masterKey, masterSalt)
	if err != nil {
		return nil, err
	}

	return &AudioConn{
		conn:    conn,
		typ:     dynamicType,
		ssrc:    2541098698, // must match SDP answer (hard-coded for now)
		seq:     uint16(rand.Uint32()),
		time:    uint32(rand.Uint32()),
		context: ctx,
	}, nil
}

// Close the underlying connection.
func (c *AudioConn) Close() {
	c.conn.Close()
}

func (c *AudioConn) Send(b []byte) error {
	var err error
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

	c.time += 960

	return err
}
