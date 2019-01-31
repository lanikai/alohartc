package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCandidate(t *testing.T) {
	desc := "candidate:0 1 UDP 123456789 192.168.1.1 12345 typ host"
	var c Candidate
	err := parseCandidateSDP(desc, &c)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, c.foundation, "0")
	assert.Equal(t, c.component, 1)
	assert.Equal(t, c.address.protocol, UDP)
	assert.Equal(t, c.address.ip.String(), "192.168.1.1")
	assert.Equal(t, c.address.port, 12345)
	assert.EqualValues(t, c.priority, 123456789)
	assert.Equal(t, c.typ, "host")
}

func TestCandidateString(t *testing.T) {
	desc := "candidate:0 1 udp 123456789 192.168.1.1 12345 typ host"
	var c Candidate
	parseCandidateSDP(desc, &c)

	assert.Equal(t, c.String(), desc)
}
