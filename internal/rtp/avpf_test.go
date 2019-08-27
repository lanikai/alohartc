package rtp

import (
	"testing"
)

func TestNACK(t *testing.T) {
	var nack nackFeedbackMessage

	nack.setLostPackets([]uint16{5, 6, 10})
	if nack.pid != 5 {
		t.Errorf("expected NACK PID = 5, not %d", nack.pid)
	}
	if nack.blp != 0x11 { // 6 -> bit at position 0, 10 -> bit at position 4
		t.Errorf("expected NACK BLP = 0x11, not 0x%x", nack.blp)
	}

	lost := nack.getLostPackets()
	if len(lost) != 3 || lost[0] != 5 || lost[1] != 6 || lost[2] != 10 {
		t.Errorf("unexpected NACK lost packets: %v", lost)
	}
}
