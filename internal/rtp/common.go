package rtp

// common.go contains generic logic that is common between RTP and RTCP (i.e.
// the data protocol and the control protocol).

import (
	"encoding/binary"
	"fmt"
)

const (
	// RFC 3550 defines RTP version 2.
	rtpVersion = 2
)

type errBadVersion byte

func (e errBadVersion) Error() string {
	return fmt.Sprintf("invalid RTP version: %d", byte(e))
}

// Demultiplex RTP/RTCP. See https://tools.ietf.org/html/rfc5761#section-4.
func identifyPacket(buf []byte) (rtcp bool, ssrc uint32, err error) {
	if len(buf) < 8 {
		err = fmt.Errorf("short RTP/RTCP packet: %02x", buf)
		return
	}
	packetType := buf[1]
	if 192 <= packetType && packetType <= 223 {
		rtcp = true
		ssrc = binary.BigEndian.Uint32(buf[4:8])
	} else {
		if len(buf) < 12 {
			err = fmt.Errorf("short RTP packet: %02x", buf)
			return
		}
		rtcp = false
		ssrc = binary.BigEndian.Uint32(buf[8:12])
	}
	return
}
