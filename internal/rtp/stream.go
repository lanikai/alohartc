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

	// RTP state for outgoing data.
	rtpOut *rtpWriter

	// RTP state for incoming data.
	rtpIn *rtpReader

	// RTCP state for outgoing control packets.
	rtcpOut *rtcpWriter

	// RTCP state for incoming control packets.
	rtcpIn *rtcpReader
}

func newStream(session *Session, opts StreamOptions) *Stream {
	// TODO: Validate options.
	s := new(Stream)
	s.StreamOptions = opts
	if opts.Direction == "sendonly" || opts.Direction == "sendrecv" {
		s.rtpOut = newRTPWriter(session.DataConn, opts.LocalSSRC, session.writeContext)
	}
	if opts.Direction == "recvonly" || opts.Direction == "sendrecv" {
		s.rtpIn = newRTPReader(opts.RemoteSSRC, session.readContext)
	}
	s.rtcpOut = newRTCPWriter(session.ControlConn, opts.LocalSSRC, session.writeContext)
	s.rtcpIn = newRTCPReader(opts.RemoteSSRC, session.readContext)
	return s
}

func (s *Stream) Close() error {
	s.sendGoodbye("stream closed")
	s.rtpOut = nil
	s.rtpIn = nil
	return nil
}

func (s *Stream) sendSenderReport() error {
	sdes := &rtcpSourceDescription{
		ssrc:  s.LocalSSRC,
		cname: s.LocalCNAME,
	}
	return s.rtcpOut.writePacket(sdes)
}

func (s *Stream) sendReceiverReport() error {
	rr := &rtcpReceiverReport{
		receiver: s.LocalSSRC,
		reports: []rtcpReport{{
			Source:       s.RemoteSSRC,
			LastReceived: uint32(s.rtpIn.lastIndex),
			// TODO: Jitter, arrival delay, etc.
		}},
	}
	sdes := &rtcpSourceDescription{
		ssrc:  s.LocalSSRC,
		cname: s.LocalCNAME,
	}
	return s.rtcpOut.writePacket(rr, sdes)
}

// Send RTCP Goodbye packet to inform the remote peer that we're leaving.
func (s *Stream) sendGoodbye(reason string) error {
	rr := &rtcpReceiverReport{
		receiver: s.LocalSSRC,
	}
	sdes := &rtcpSourceDescription{
		ssrc:  s.LocalSSRC,
		cname: s.LocalCNAME,
	}
	bye := &rtcpGoodbye{
		ssrc:   s.LocalSSRC,
		reason: reason,
	}
	return s.rtcpOut.writePacket(rr, sdes, bye)
}
