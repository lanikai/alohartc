package rtp

import (
	"io"
	"net"
)

type SessionOptions struct {
	// SRTP master key material.
	ReadKey   []byte
	ReadSalt  []byte
	WriteKey  []byte
	WriteSalt []byte

	// Maximum size of outgoing packets, factoring in MTU and protocol overhead.
	MaxPacketSize int
}

const (
	// It's hard to find authoritative information, but according to a popular
	// StackOverflow answer, a 512-byte UDP payload is generally considered safe
	// (https://stackoverflow.com/a/1099359/11194515).
	defaultMaxPacketSize = 512
)

// A Session represents an established RTP/RTCP connection to a remote peer. It
// contains one or more streams, each represented by their own SSRC.
type Session struct {
	SessionOptions

	conn net.Conn

	// RTP streams in this session, keyed by SSRC. Every stream appears twice in
	// the map, once for the local SSRC and once for the remote SSRC.
	streams map[uint32]*Stream

	// SRTP cryptographic contexts.
	readContext  *cryptoContext
	writeContext *cryptoContext
}

func NewSession(conn net.Conn, opts SessionOptions) *Session {
	if opts.MaxPacketSize == 0 {
		opts.MaxPacketSize = defaultMaxPacketSize
	}

	s := new(Session)
	s.SessionOptions = opts
	s.conn = conn
	s.streams = make(map[uint32]*Stream)
	if opts.ReadKey != nil && opts.ReadSalt != nil {
		s.readContext = newCryptoContext(opts.ReadKey, opts.ReadSalt)
	}
	if opts.WriteKey != nil && opts.WriteSalt != nil {
		s.writeContext = newCryptoContext(opts.WriteKey, opts.WriteSalt)
	}
	go s.readLoop()
	return s
}

func (s *Session) Close() error {
	return s.conn.Close()
}

func (s *Session) AddStream(opts StreamOptions) *Stream {
	if opts.MaxPacketSize == 0 {
		opts.MaxPacketSize = s.MaxPacketSize
	}
	stream := newStream(s, opts)
	s.streams[stream.LocalSSRC] = stream
	s.streams[stream.RemoteSSRC] = stream
	return stream
}

func (s *Session) RemoveStream(stream *Stream) {
	delete(s.streams, stream.LocalSSRC)
	delete(s.streams, stream.RemoteSSRC)
}

// Returns on read error or when the session is closed.
func (s *Session) readLoop() {
	buf := make([]byte, 65536)
	for {
		n, err := s.conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Debug("read RTP: EOF")
			} else {
				log.Error("read RTP: %v", err)
			}
			return
		}

		pkt := buf[0:n]
		rtcp, ssrc, err := identifyPacket(pkt)
		if err != nil {
			log.Error("read RTP: %v", err)
			return
		}

		stream := s.streams[ssrc]
		if stream == nil {
			log.Debug("read RTP: unknown SSRC %02x", ssrc)
			continue
		}

		if rtcp {
			//	stream.handleRTCP(pkt)
		} else {
			if err := stream.rtpIn.readPacket(pkt); err != nil {
				log.Error("read RTP: %v", err)
			}
		}
	}
}
