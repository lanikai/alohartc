package rtp

// Payload type description, as provided via SDP.
type PayloadType struct {
	// Payload type number (<= 127) assigned by the SDP `rtpmap` attribute.
	Number uint8

	// Encoding name, from the SDP `rtpmap` attribute (e.g. "H264").
	Name string

	// Clock rate in Hz, from the SDP `rtpmap` attribute (e.g. 90000).
	ClockRate int

	// Codec-specific format parameters, from the SDP `fmtp` attribute.
	Format string

	// Supported feedback RTCP options, from the SDP `rtcp-fb` attributes.
	FeedbackOptions []string
}

type StreamOptions struct {
	LocalSSRC  uint32
	LocalCNAME string

	RemoteSSRC  uint32
	RemoteCNAME string

	// sendonly, recvonly, or sendrecv
	Direction string

	// Negotiated payload types, keyed by 7-bit dynamic payload type number.
	PayloadTypes map[byte]PayloadType

	// Maximum size of outgoing packets, factoring in MTU and protocol overhead.
	MaxPacketSize int
}

type Stream struct {
	StreamOptions

	session *Session

	// RTP sender state for outgoing data streams.
	rtpOut *rtpWriter

	// RTP receiver state for incoming data streams.
	rtpIn *rtpReader

	// RTCP send state

	// RTCP receive state
}

func newStream(session *Session, opts StreamOptions) *Stream {
	// TODO: Validate options.
	s := new(Stream)
	s.StreamOptions = opts
	s.session = session
	if opts.Direction == "sendonly" || opts.Direction == "sendrecv" {
		s.rtpOut = newRTPWriter(session.conn, opts.LocalSSRC, session.writeContext)
	}
	if opts.Direction == "recvonly" || opts.Direction == "sendrecv" {
		s.rtpIn = newRTPReader(opts.RemoteSSRC, session.readContext)
	}
	return s
}

func (s *Stream) Close() error {
	if s.rtpOut != nil {
		// TODO: Send RTCP Goodbye packet.
		//if err := s.rtcpOut.Goodbye(); err != nil {
		//	return err
		//}
		s.rtpOut = nil
	}
	s.rtpIn = nil
	return nil
}

// TODO; rtpIn/rtcpIn consumers
