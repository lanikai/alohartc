package srtp

import (
	"testing"
)

func TestSRTPMarshalBinary(t *testing.T) {
	srtp := SRTP{
		Marker:         true,
		PayloadType:    96,
		SequenceNumber: 1234,
		SSRC:           0x20180709,
		CSRC: []uint32{
			0x20180709,
			0x20180709,
		},
	}

	_, err := srtp.MarshalBinary()
	if err != nil {
		t.Fail()
	}
}
