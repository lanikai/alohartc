package ice

import (
	"errors"
	"math"
	"net"
	"time"
)

// Implements net.Conn with channels.
type ChannelConn struct {
	in    <-chan []byte // Channel for reads
	out   chan<- []byte // Channel for writes
	laddr net.Addr      // Local address
	raddr net.Addr      // Remote address
	rtimer *time.Timer  // Timer to enforce read deadline
	wtimer *time.Timer  // Timer to enforce write deadline
	closed chan bool
}

const never = math.MaxInt64

func newChannelConn(in <-chan []byte, out chan<- []byte, laddr, raddr net.Addr) *ChannelConn {
	c := &ChannelConn{}
	c.in = in
	c.out = out
	c.laddr = laddr
	c.raddr = raddr
	c.rtimer = time.NewTimer(never)
	c.wtimer = time.NewTimer(never)
	c.closed = make(chan bool, 1)
	return c
}

func (c *ChannelConn) Read(b []byte) (n int, err error) {
	select {
	case <-c.closed:
		c.closed <- true
		err = errors.New("Channel closed during read")
	case data, ok := <-c.in:
		if ok {
			// TODO: Deal with case where data doesn't fit in b
			n = len(data)
			copy(b, data)
		}
	case <-c.rtimer.C:
		err = errors.New("Read timeout")
	}
	return
}

func (c *ChannelConn) Write(b []byte) (n int, err error) {
	select {
	case <-c.closed:
		c.closed <- true
		err = errors.New("Channel closed during write")
	case c.out <- b:
		n = len(b)
	case <-c.wtimer.C:
		err = errors.New("Write timeout")
	}
	return
}

func (c *ChannelConn) Close() error {
	switch {
	case <-c.closed:
		c.closed <- true
	default:
		// First time closing, so also close the in channel.
		close(c.out)
		c.closed <- true
	}
	return nil
}

func (c *ChannelConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *ChannelConn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *ChannelConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (c *ChannelConn) SetReadDeadline(t time.Time) error {
	var d time.Duration
	if t.IsZero() {
		d = never
	} else {
		d = time.Until(t)
	}
	if !c.rtimer.Stop() {
		// Timer already expired. Drain the channel.
		select {
		case <-c.rtimer.C:
		default:
		}
	}
	c.rtimer.Reset(d)
	return nil
}

func (c *ChannelConn) SetWriteDeadline(t time.Time) error {
	var d time.Duration
	if t.IsZero() {
		d = never
	} else {
		d = time.Until(t)
	}
	if !c.wtimer.Stop() {
		// Timer already expired. Drain the channel.
		select {
		case <-c.wtimer.C:
		default:
		}
	}
	c.wtimer.Reset(d)
	return nil
}
