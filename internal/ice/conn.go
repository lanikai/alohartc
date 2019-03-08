package ice

import (
	"errors"
	"io"
	"math"
	"net"
	"time"
)

// Implements net.Conn with channels.
type ChannelConn struct {
	// Underlying UDP connection
	conn *net.UDPConn

	in     <-chan []byte // Channel for reads
	laddr  net.Addr      // Local address
	raddr  net.Addr      // Remote address
	rtimer *time.Timer   // Timer to enforce read deadline
}

// newChannelConn returns a new initialized channel-based connection
func NewChannelConn(base *Base, in <-chan []byte, raddr net.Addr) *ChannelConn {
	return &ChannelConn{
		conn:   base.UDPConn,
		in:     in,
		raddr:  raddr,
		rtimer: time.NewTimer(math.MaxInt64),
	}
}

// Read next buffer from connection. If closed, returns with n = 0.
func (c *ChannelConn) Read(b []byte) (int, error) {
	select {
	case data, ok := <-c.in:
		if !ok {
			return 0, io.EOF
		}

		if len(data) > len(b) {
			log.Warn("read truncated due to short buffer")
		}

		copy(b, data)

		return len(data), nil

	case <-c.rtimer.C:
		return 0, errors.New("read timeout")
	}
}

// Write buffer to connection. If closed, returns with n = 0.
func (c *ChannelConn) Write(b []byte) (int, error) {
	return c.conn.WriteTo(b, c.raddr)
}

func (c *ChannelConn) Close() error {
	return nil
}

func (c *ChannelConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *ChannelConn) RemoteAddr() net.Addr {
	return c.raddr
}

// SetDeadline sets both the read and write timeouts
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
	// Stop timer, and, if already stopped, drain channel
	if !c.rtimer.Stop() {
		select {
		// Prevent timer from firing after call to rtimer.Stop()
		case <-c.rtimer.C:

		// If timer stopped by a previous call, reading from rtimer.C would
		// block. This default case prevents deadlock.
		default:
		}
	}

	// Reset timer, if a non-zero deadline specified
	if !t.IsZero() {
		c.rtimer.Reset(time.Until(t))
	}

	return nil
}

// SetWriteDeadline sets a write timeout
func (c *ChannelConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
