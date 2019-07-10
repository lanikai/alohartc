package srtp

import "testing"

func TestMarshalAndUnmarshal(t *testing.T) {
	testMessageIn := rtpMsg{
		marker:         true,
		payloadType:    100,
		sequenceNumber: 12345,
		timestamp:      12345,
		ssrc:           0xdeadbeef,
		csrc: []uint32{
			0xdeadbeef,
		},
		payload: []byte{
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		},
	}
	marshaled := testMessageIn.marshal()

	var testMessageOut rtpMsg

	if err := testMessageOut.unmarshal(marshaled); err != nil {
		t.Fail()
	}

	if (testMessageIn.marker != testMessageOut.marker) ||
		(testMessageIn.payloadType != testMessageOut.payloadType) ||
		(testMessageIn.sequenceNumber != testMessageOut.sequenceNumber) ||
		(testMessageIn.timestamp != testMessageOut.timestamp) ||
		(testMessageIn.ssrc != testMessageOut.ssrc) ||
		len(testMessageIn.csrc) != len(testMessageOut.csrc) {
		t.Fail()
	}
}
