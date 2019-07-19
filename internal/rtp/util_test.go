package rtp

import "testing"

func TestSplit215(t *testing.T) {
	b2, b1, b5 := splitByte215(0x80 | 0x20 | 0x05)
	if b2 != 2 {
		t.Fail()
	}
	if !b1 {
		t.Fail()
	}
	if b5 != 5 {
		t.Fail()
	}
}

func TestSplit17(t *testing.T) {
	b1, b7 := splitByte17(0x80 | 0x35)
	if !b1 {
		t.Fail()
	}
	if b7 != 0x35 {
		t.Fail()
	}
}

func TestTrunc(t *testing.T) {
	a := uint64(0x1ff)
	if trunc(a, 8) != 0xff {
		t.Fail()
	}
	if trunc(a, 7) != 0x7f {
		t.Fail()
	}
}
