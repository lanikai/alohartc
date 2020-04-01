package ice

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransportAddressIPv4(t *testing.T) {
	ta := makeTransportAddress(&net.UDPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 5678,
	})

	assert.True(t, ta.resolved())
	assert.Equal(t, IPv4, ta.family)
	assert.Equal(t, []byte{1, 2, 3, 4}, []byte(ta.ip))
	assert.Equal(t, "1.2.3.4", ta.displayIP())
	assert.Equal(t, "udp/1.2.3.4:5678", ta.String())
}

func TestTransportAddressIPv6(t *testing.T) {
	ta := makeTransportAddress(&net.UDPAddr{
		IP:   net.ParseIP("1:2:3:4::"),
		Port: 5678,
	})

	assert.True(t, ta.resolved())
	assert.Equal(t, IPv6, ta.family)
	assert.Equal(t, []byte{0, 1, 0, 2, 0, 3, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0}, []byte(ta.ip))
	assert.Equal(t, "1:2:3:4::", ta.displayIP())
	assert.Equal(t, "udp/[1:2:3:4::]:5678", ta.String())
}

func TestTransportAddressUnresolved(t *testing.T) {
	ta := TransportAddress{
		protocol: UDP,
		ip:       IPAddress("foo.local"),
		port:     5678,
	}

	assert.False(t, ta.resolved())
	assert.Equal(t, Unresolved, ta.family)
	assert.Equal(t, IPAddress("foo.local"), ta.ip)
	assert.Equal(t, "foo.local", ta.displayIP())
	assert.Equal(t, "udp/foo.local:5678", ta.String())
}
