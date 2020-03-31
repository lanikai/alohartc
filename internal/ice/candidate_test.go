package ice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCandidate(t *testing.T) {
	desc := "candidate:0 1 UDP 123456789 192.168.1.1 12345 typ host"
	c, err := ParseCandidate(desc, "mid")
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "0", c.foundation)
	assert.Equal(t, 1, c.component)
	assert.Equal(t, UDP, c.address.protocol)
	assert.Equal(t, IPAddress("\xc0\xa8\x01\x01"), c.address.ip)
	assert.Equal(t, "192.168.1.1", c.address.displayIP())
	assert.Equal(t, 12345, c.address.port)
	assert.Equal(t, uint32(123456789), c.priority)
	assert.Equal(t, "host", c.typ)
}

func TestCandidateString(t *testing.T) {
	desc := "candidate:0 1 udp 123456789 192.168.1.1 12345 typ host"
	c, _ := ParseCandidate(desc, "mid")

	assert.Equal(t, desc, c.String())
}
