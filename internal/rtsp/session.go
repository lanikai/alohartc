package rtsp

import ()

type Session struct {
	id        string
	transport *Transport
}

func (s *Session) Close() error {
	return s.transport.Close()
}
