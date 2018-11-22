package webrtc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseOrigin(t *testing.T) {
	o, err := parseOrigin("o=username id 123 IN IP4 0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, o.username, "username")
	assert.Equal(t, o.sessionId, "id")
	assert.EqualValues(t, o.sessionVersion, 123)
	assert.Equal(t, o.networkType, "IN")
	assert.Equal(t, o.addressType, "IP4")
	assert.Equal(t, o.address, "0.0.0.0")
}

func TestWriteOrigin(t *testing.T) {
	o, _ := parseOrigin("o=username id 123 IN IP4 0.0.0.0")
	assert.Equal(t, o.String(), "o=username id 123 IN IP4 0.0.0.0")
}

func TestParseSession(t *testing.T) {
	sdp := `v=0
o=- 6830938501909068252 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE sdparta_0
a=msid-semantic: WMS SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
m=video 9 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 102 123 127 122 125 107 108 109 124
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:n3E3
a=ice-pwd:auh7I7RsuhlZQgS2XYLStR05
a=ice-options:trickle
a=fingerprint:sha-256 05:67:ED:76:91:C6:58:F3:01:CE:F2:01:6A:04:10:53:C3:B3:9A:74:49:68:18:D5:60:D0:BC:25:1B:95:9C:50
a=setup:active
a=mid:sdparta_0
a=sendonly
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:100 H264/90000
a=rtcp-fb:100 goog-remb
a=rtcp-fb:100 transport-cc
a=rtcp-fb:100 ccm fir
a=rtcp-fb:100 nack
a=rtcp-fb:100 nack pli
a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:102 H264/90000
a=rtcp-fb:102 goog-remb
a=rtcp-fb:102 transport-cc
a=rtcp-fb:102 ccm fir
a=rtcp-fb:102 nack
a=rtcp-fb:102 nack pli
a=fmtp:102 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:123 rtx/90000
a=fmtp:123 apt=102
a=rtpmap:127 H264/90000
a=rtcp-fb:127 goog-remb
a=rtcp-fb:127 transport-cc
a=rtcp-fb:127 ccm fir
a=rtcp-fb:127 nack
a=rtcp-fb:127 nack pli
a=fmtp:127 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=4d0032
a=rtpmap:122 rtx/90000
a=fmtp:122 apt=127
a=rtpmap:125 H264/90000
a=rtcp-fb:125 goog-remb
a=rtcp-fb:125 transport-cc
a=rtcp-fb:125 ccm fir
a=rtcp-fb:125 nack
a=rtcp-fb:125 nack pli
a=fmtp:125 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032
a=rtpmap:107 rtx/90000
a=fmtp:107 apt=125
a=rtpmap:108 red/90000
a=rtpmap:109 rtx/90000
a=fmtp:109 apt=108
a=rtpmap:124 ulpfec/90000
a=ssrc-group:FID 2541098696 3215547008
a=ssrc:2541098696 cname:cYhx/N8U7h7+3GW3
a=ssrc:2541098696 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:2541098696 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
a=ssrc:2541098696 label:e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:3215547008 cname:cYhx/N8U7h7+3GW3
a=ssrc:3215547008 msid:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv e9b60276-a415-4a66-8395-28a893918d4c
a=ssrc:3215547008 mslabel:SdWLKyaNRoUSWQ7BzkKGcbCWcuV7rScYxCAv
a=ssrc:3215547008 label:e9b60276-a415-4a66-8395-28a893918d4c
`
	s, err := parseSession(sdp)
	if err != nil {
		t.Fatal(err)
	}
	assert.EqualValues(t, s.version, 0)
	assert.Equal(t, s.name, "-")

	o := s.origin
	assert.Equal(t, o.username, "-")
	assert.Equal(t, o.sessionId, "6830938501909068252")
	assert.EqualValues(t, o.sessionVersion, 2)
	assert.EqualValues(t, o.networkType, "IN")
	assert.EqualValues(t, o.addressType, "IP4")
	assert.EqualValues(t, o.address, "127.0.0.1")

	assert.Len(t, s.media, 1)
	m := s.media[0]

	c := m.connection
	assert.NotNil(t, c)
	assert.Equal(t, c.networkType, "IN")
	assert.Equal(t, c.addressType, "IP4")
	assert.Equal(t, c.address, "0.0.0.0")
}

func TestWriteSession(t *testing.T) {
	s := SessionDesc{
		version: 0,
		origin: OriginDesc{
			username:       "fred",
			sessionId:      "123",
			sessionVersion: 9,
			networkType:    "IN",
			addressType:    "IP4",
			address:        "127.0.0.1",
		},
		name: "mysession",
	}

	assert.Equal(t, s.String(), "v=0\r\no=fred 123 9 IN IP4 127.0.0.1\r\ns=mysession\r\n")
}
