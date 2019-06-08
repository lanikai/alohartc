package mux

import (
	"io"
	"net"
	"sync"
	"time"
)

// Endpoint implements net.Conn. It is used to read muxed packets. Incoming
// packets are delivered by the Mux, and placed in a circular queue of buffers.
// Readers grab packets from the queue as they become available.
type Endpoint struct {
	mux *Mux

	// A circular queue of buffers, each of which can hold a single data packet.
	bufs [][]byte

	// Number of buffers in the circular queue. This is just len(bufs).
	nbufs int

	// Number of buffers currently occupied with data. 0 <= nused <= nbufs.
	nused int

	// The index of the first used buffer. 0 <= first < nbufs.
	first int

	// Single-item channel indicating when there are packets waiting to be read.
	available chan struct{}

	// One-time channel indicating that the endpoint has been closed.
	dead chan struct{}

	// Mutex held when modifying circular queue state.
	sync.Mutex
}

func createEndpoint(mux *Mux, nbufs int, bufsize int) *Endpoint {
	// Create a large shared buffer pool, and split into nbuf individual buffers
	// of size bufsize.
	bufpool := make([]byte, nbufs*bufsize)
	bufs := make([][]byte, nbufs)
	for i := 0; i < nbufs; i++ {
		bufs[i] = bufpool[i*bufsize : (i+1)*bufsize]
	}
	return &Endpoint{
		mux:       mux,
		bufs:      bufs,
		nbufs:     nbufs,
		nused:     0,
		first:     0,
		available: make(chan struct{}, 1),
		dead:      make(chan struct{}),
	}
}

// Close unregisters the endpoint from the Mux
func (e *Endpoint) Close() error {
	e.close()
	e.mux.RemoveEndpoint(e)
	return nil
}

func (e *Endpoint) close() {
	e.Lock()
	select {
	case <-e.dead:
	default:
		close(e.dead)
	}
	e.Unlock()
}

// Exchange the provided buffer (containing a packet of data) with an unused
// buffer from this endpoint's circular queue.
func (e *Endpoint) deliver(buf []byte) []byte {
	e.Lock()
	defer e.Unlock()

	select {
	case <-e.dead:
		return buf
	case e.available <- struct{}{}:
	default:
	}

	if e.nused == e.nbufs {
		// All buffers are in use. Drop the oldest and add the new packet to the
		// end.
		ret := e.bufs[e.first]
		e.bufs[e.first] = buf
		e.first = (e.first + 1) % e.nbufs
		return ret
	} else {
		// Swap the new packet with the next unused buffer in the queue.
		next := (e.first + e.nused) % e.nbufs
		ret := e.bufs[next]
		e.bufs[next] = buf
		e.nused++
		return ret
	}
}

// If there are packets available, copy the first available one into p.
func (e *Endpoint) tryConsume(p []byte) (int, error, bool) {
	e.Lock()
	defer e.Unlock()

	if e.nused == 0 {
		return 0, nil, false
	}

	// Copy first used buffer to p, and advance e.first.
	n := copy(p, e.bufs[e.first])
	e.first = (e.first + 1) % e.nbufs
	e.nused--

	// Keep the available channel full if more packets are available.
	if e.nused > 0 {
		select {
		case e.available <- struct{}{}:
		default:
		}
	}

	return n, nil, true
}

// Read reads a packet of len(p) bytes from the underlying conn
// that are matched by the associated MuxFunc
func (e *Endpoint) Read(p []byte) (int, error) {
	if e.nused > 0 {
		// There's a packet waiting. Try to consume it right away.
		n, err, ok := e.tryConsume(p)
		if ok {
			return n, err
		}
	}

	// Otherwise, wait for a packet to arrive. Avoid racing with other readers.
	for {
		select {
		case <-e.dead:
			return 0, io.EOF
		case <-e.available:
			n, err, ok := e.tryConsume(p)
			if ok {
				return n, err
			}
		}
	}
}

// Write writes len(p) bytes to the underlying conn
func (e *Endpoint) Write(p []byte) (n int, err error) {
	return e.mux.nextConn.Write(p)
}

// LocalAddr is a stub
func (e *Endpoint) LocalAddr() net.Addr {
	return e.mux.nextConn.LocalAddr()
}

// RemoteAddr is a stub
func (e *Endpoint) RemoteAddr() net.Addr {
	return e.mux.nextConn.RemoteAddr()
}

// SetDeadline is a stub
func (e *Endpoint) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is a stub
func (e *Endpoint) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a stub
func (e *Endpoint) SetWriteDeadline(t time.Time) error {
	return nil
}
