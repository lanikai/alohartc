package webrtc

import (
	"testing"
)

func TestParseIceCandidate(t *testing.T) {
	c, err := parseCandidate("candidate:0 1 UDP 1111111111 192.168.1.1 12345 typ host")
	if err != nil {
		t.Error(err)
	}
	t.Log(c)
}
