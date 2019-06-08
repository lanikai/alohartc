package mux

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDispatch(t *testing.T) {
	m := &Mux{
		endpoints: make(map[*Endpoint]MatchFunc),
	}
	e := m.NewEndpoint(MatchRange(0, 255))

	if e.nused != 0 {
		t.Errorf("Expected endpoint to have 0 used buffers: %d", e.nused)
	}

	// Dispatch one packet to the endpoint.
	pkt := []byte("test")
	ret := m.dispatch(pkt)

	if e.nused != 1 {
		t.Errorf("Expected endpoint to have 1 used buffer after dispatch: %d", e.nused)
	}
	if !identical(e.bufs[0], pkt) {
		t.Errorf("Expected endpoint to have taken ownership of packet buffer: %p != %p", &e.bufs[0], &pkt)
	}
	if identical(ret, pkt) {
		t.Errorf("Expected dispatch to receive a different buffer")
	}

	// Read the packet out of the endpoint.
	buf := make([]byte, 32)
	n, err := e.Read(buf)

	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(buf[:n], pkt) {
		t.Errorf("Read: unexpected value: %q != %q", buf[:n], pkt)
	}
	if e.nused != 0 {
		t.Errorf("Expected endpoint to have 0 used buffers after Read: %d", e.nused)
	}
}

// Checks if two byte slices refer to the exact same memory region.
func identical(b1, b2 []byte) bool {
	return len(b1) == len(b2) &&
		reflect.ValueOf(b1).Pointer() == reflect.ValueOf(b2).Pointer()
}
