package mux

import (
	"fmt"
	"net"
	"sync"
)

const (
	// Number of packets to buffer for each endpoint.
	numBufferPackets = 32
)

// Mux allows multiplexing
type Mux struct {
	lock       sync.Mutex
	nextConn   net.Conn
	endpoints  map[*Endpoint]MatchFunc
	bufferSize int
}

// NewMux creates a new Mux. This Mux takes ownership of the underlying
// net.Conn, and is responsible for closing it.
func NewMux(conn net.Conn, bufferSize int) *Mux {
	m := &Mux{
		nextConn:   conn,
		endpoints:  make(map[*Endpoint]MatchFunc),
		bufferSize: bufferSize,
	}

	go m.readLoop()

	return m
}

// NewEndpoint creates a new Endpoint
func (m *Mux) NewEndpoint(f MatchFunc) *Endpoint {
	e := createEndpoint(m, numBufferPackets, m.bufferSize)

	m.lock.Lock()
	m.endpoints[e] = f
	m.lock.Unlock()

	return e
}

// RemoveEndpoint removes an endpoint from the Mux
func (m *Mux) RemoveEndpoint(e *Endpoint) {
	m.lock.Lock()
	delete(m.endpoints, e)
	m.lock.Unlock()
}

// Close closes the Mux and all associated Endpoints.
func (m *Mux) Close() error {
	m.lock.Lock()
	for e := range m.endpoints {
		e.close()
		delete(m.endpoints, e)
	}
	m.lock.Unlock()

	err := m.nextConn.Close()
	if err != nil {
		return err
	}

	return nil
}

// Read continually from the underlying connection and dispatch to the
// appropriate endpoint. Terminate on read error, e.g. when the underlying
// connection is closed.
func (m *Mux) readLoop() {
	defer m.Close()

	buf := make([]byte, m.bufferSize)
	for {
		n, err := m.nextConn.Read(buf)
		if err != nil {
			return
		}

		// Dispatching to endpoints is done with a "give a penny, take a penny"
		// approach. The data packet is delivered to the endpoint in exchange
		// for one of its unused buffers.
		buf = m.dispatch(buf[:n])

		// Resize the buffer to its full capacity (m.bufferSize), since we may
		// have shrunk it when we originally dispatched it to the endpoint.
		buf = buf[0:cap(buf)]
	}
}

func (m *Mux) dispatch(buf []byte) []byte {
	var endpoint *Endpoint

	m.lock.Lock()
	for e, f := range m.endpoints {
		if f(buf) {
			endpoint = e
			break
		}
	}
	m.lock.Unlock()

	if endpoint == nil {
		fmt.Printf("Warning: mux: no endpoint for packet starting with %d\n", buf[0])
		return buf
	}

	return endpoint.deliver(buf)
}
