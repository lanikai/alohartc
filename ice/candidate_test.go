package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCandidate(t *testing.T) {
	desc := "candidate:0 1 UDP 123456789 192.168.1.1 12345 typ host"
	c, err := parseCandidate(desc)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, c.foundation, "0")
	assert.Equal(t, c.component, 1)
	assert.Equal(t, c.protocol, "UDP")
	assert.EqualValues(t, c.priority, 123456789)
	assert.Equal(t, c.ip, "192.168.1.1")
	assert.Equal(t, c.port, 12345)
	assert.Equal(t, c.typ, "host")
}

func TestCandidateString(t *testing.T) {
	desc := "candidate:0 1 UDP 123456789 192.168.1.1 12345 typ host"
	c, _ := parseCandidate(desc)

	assert.Equal(t, c.String(), desc)
}
