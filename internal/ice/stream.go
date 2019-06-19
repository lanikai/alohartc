package ice

import (
	"errors"
	"io"
	"net"
	"time"
)

var ErrReadTimeout = errors.New("read timeout")

// A DataStream represents an active connection between two peers. ICE control
// packets are filtered out and handled by the ICE agent, so reads and writes
// deal exclusively with the data-level protocol.
type DataStream struct {
	// Parent connection. Write operations pass through to the parent, read
	// operations use the in channel.
	conn net.PacketConn

	// Remote address that this data stream writes to.
	raddr net.Addr

	// Inbound packet stream, fed by a read loop on the parent connection.
	in <-chan []byte

	// Single-fire channel used to indicate that the read loop has terminated.
	dead <-chan struct{}

	// Function that returns why the read loop died.
	cause func() error

	// Timer used to implement read deadlines.
	timer *time.Timer

	// Signal channel used to notify pending reads when the deadline changes.
	notify chan struct{}
}

// Return a channel that fires when the data session is severed.
func (s *DataStream) Done() <-chan struct{} {
	return s.dead
}

func (s *DataStream) Err() error {
	return s.cause()
}

func (s *DataStream) Write(b []byte) (int, error) {
	return s.conn.WriteTo(b, s.raddr)
}

func (s *DataStream) Read(b []byte) (int, error) {
	if s.notify == nil {
		s.notify = make(chan struct{})
	}

	for {
		var timeout <-chan time.Time
		if s.timer != nil {
			timeout = s.timer.C
		}

		select {
		case <-s.notify:
			continue
		case <-s.dead:
			return 0, io.EOF
		case <-timeout:
			return 0, ErrReadTimeout
		case data := <-s.in:
			n := len(data)
			if n > len(b) {
				// For packet-oriented connections, the destination buffer must
				// be large enough to fit an entire packet.
				return 0, io.ErrShortBuffer
			}

			copy(b, data)
			return n, nil
		}
	}
}

func (s *DataStream) Close() error {
	return s.conn.Close()
}

func (s *DataStream) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *DataStream) RemoteAddr() net.Addr {
	return s.raddr
}

func (s *DataStream) SetDeadline(t time.Time) error {
	if err := s.SetReadDeadline(t); err != nil {
		return err
	}

	return s.SetWriteDeadline(t)
}

func (s *DataStream) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

// See net.Conn#SetReadDeadline for semantics.
func (s *DataStream) SetReadDeadline(t time.Time) error {
	if s.timer != nil {
		if !s.timer.Stop() {
			// Drain the channel if we were unable to stop the timer before it
			// fired. This may race with pending reads, so use best effort.
			select {
			case <-s.timer.C:
			default:
			}
		}
	}

	if t.IsZero() {
		// Zero deadline means no timeout.
		s.timer = nil
	} else if s.timer == nil {
		s.timer = time.NewTimer(time.Until(t))
	} else {
		s.timer.Reset(time.Until(t))
	}

	// Notify pending reads of the new deadline.
	n := s.notify
	s.notify = make(chan struct{})
	if n != nil {
		close(n)
	}
	return nil
}
