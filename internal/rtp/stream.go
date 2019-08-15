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

	// RTP state for outgoing data.
	rtpOut *rtpWriter

	// RTP state for incoming data.
	rtpIn *rtpReader

	// RTCP state for outgoing control packets.
	rtcpOut *rtcpWriter

	// RTCP state for incoming control packets.
	//rtcpIn *rtcpReader
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
	s.rtcpOut = newRTCPWriter(session.conn, opts.LocalSSRC, session.writeContext)
	//s.rtcpIn = newRTCPReader(session.conn, opts.LocalSSRC, session.readContext)
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

func (s *Stream) sendReceiverReport() error {
	rr := &rtcpReceiverReport{
		receiver: s.LocalSSRC,
		reports: []rtcpReport{{
			Source:       s.RemoteSSRC,
			LastReceived: uint32(s.rtpIn.lastIndex),
		}},
	}

	sdes := &rtcpSourceDescription{
		ssrc: s.LocalSSRC,
	}

	return s.rtcpOut.writePackets(rr, sdes)
}

// TODO; rtpIn/rtcpIn consumers
