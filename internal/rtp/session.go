package rtp

import (
	"io"
	"net"
)

type SessionOptions struct {
	// Single connection over which RTP and RTCP are muxed.
	MuxConn net.Conn

	// Separate connections for RTP and RTCP.
	DataConn    net.Conn
	ControlConn net.Conn

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

	// RTP streams in this session, keyed by SSRC. Every stream appears twice in
	// the map, once for the local SSRC and once for the remote SSRC.
	streams map[uint32]*Stream

	// SRTP cryptographic contexts.
	readContext  *cryptoContext
	writeContext *cryptoContext
}

func NewSession(opts SessionOptions) *Session {
	if opts.MaxPacketSize == 0 {
		opts.MaxPacketSize = defaultMaxPacketSize
	}

	s := &Session{
		SessionOptions: opts,
		streams:        make(map[uint32]*Stream),
	}

	if opts.ReadKey != nil && opts.ReadSalt != nil {
		s.readContext = newCryptoContext(opts.ReadKey, opts.ReadSalt)
	}
	if opts.WriteKey != nil && opts.WriteSalt != nil {
		s.writeContext = newCryptoContext(opts.WriteKey, opts.WriteSalt)
	}

	if s.MuxConn != nil {
		// Mux RTP and RTCP over a single connection.
		s.DataConn = s.MuxConn
		s.ControlConn = s.MuxConn
		go s.readLoop(s.MuxConn)
	} else {
		go s.readLoop(s.DataConn)
		go s.readLoop(s.ControlConn)
	}
	return s
}

func (s *Session) Close() error {
	err := s.DataConn.Close()
	if s.ControlConn != s.DataConn {
		if err2 := s.ControlConn.Close(); err == nil && err2 != nil {
			err = err2
		}
	}
	return err
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

// Reads packets from conn. Returns on read error or when conn is closed.
func (s *Session) readLoop(conn net.Conn) {
	buf := make([]byte, 65536)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Debug("RTP session: EOF")
			} else {
				log.Error("RTP session: %v", err)
			}
			return
		}

		pkt := buf[0:n]
		rtcp, ssrc, err := identifyPacket(pkt)
		if err != nil {
			log.Error("RTP session: %v", err)
			return
		}

		stream := s.streams[ssrc]
		if stream == nil {
			log.Debug("RTP session: unknown SSRC %02x", ssrc)
			continue
		}

		if rtcp {
			if err := stream.rtcpIn.readPacket(pkt); err != nil {
				log.Error("RTP session: %v", err)
			}
		} else {
			if err := stream.rtpIn.readPacket(pkt); err != nil {
				log.Error("RTP session: %v", err)
			}
		}
	}
}
