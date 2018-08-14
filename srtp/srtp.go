package srtp

import (
	"net"
)

type Conn struct {
	conn net.Conn
}

type Config struct {
	ssrc uint32
}

func Open() (Conn, error) {
}

func (c *Conn) Write(p []byte) (n int, err error) {
}
