package rtp

import (
	"io"

	errors "golang.org/x/xerrors"

	"github.com/lanikai/alohartc/internal/packet"
)

const (
	rtcpHeaderSize = 4
	rtcpReportSize = 6 * 4

	// From RFC 3550 Section 6.
	rtcpSenderReportType      = 200
	rtcpReceiverReportType    = 201
	rtcpSourceDescriptionType = 202
	rtcpGoodbyeType           = 203
	rtcpAppType               = 204
)

// RTP Control Protocol (RTCP), as defined in RFC 3550 Section 6.

// RTCP packets come in several different types. While they differ structurally,
// they all share a common 4-byte prefix header (where the meaning of count
// depends on packet type). See https://tools.ietf.org/html/rfc3550#section-6.
//    0                   1                   2                   3
//    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//   |V=2|P|  count  |  packet type  |             length            |
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type rtcpHeader struct {
	padding    bool
	count      int
	packetType byte
	length     int // length of RTCP packet in 32-bit words minus one
}

func (h *rtcpHeader) readFrom(r *packet.Reader) error {
	var version, count byte
	version, h.padding, count = splitByte215(r.ReadByte())
	if version != rtpVersion {
		return errBadVersion(version)
	}
	h.count = int(count)
	h.packetType = r.ReadByte()
	h.length = int(r.ReadUint16())

	return nil
}

func (h *rtcpHeader) writeTo(w *packet.Writer) error {
	if w.Capacity() < rtcpHeaderSize {
		return io.ErrShortBuffer
	}
	w.WriteByte(joinByte215(rtpVersion, h.padding, byte(h.count)))
	w.WriteByte(h.packetType)
	w.WriteUint16(uint16(h.length))
	return nil
}

type rtcpPacket interface {
	// Write the RTCP packet to the provided buffer. Return the number of bytes
	// written.
	writeTo(w *packet.Writer) error
}

// A compound RTCP packet consists of one or more RTCP packets, concatenated
// into a single datagram. See https://tools.ietf.org/html/rfc3550#section-6.1.
type rtcpCompoundPacket struct {
	packets []rtcpPacket
}

func (cp *rtcpCompoundPacket) readFrom(r *packet.Reader) error {
	if err := r.CheckRemaining(rtcpHeaderSize); err != nil {
		return errors.Errorf("short buffer: %v", err)
	}

	cp.packets = nil

	var h rtcpHeader
	for {
		if err := h.readFrom(r); err != nil {
			return err
		}

		if err := r.CheckRemaining(4 * h.length); err != nil {
			return errors.Errorf("short RTCP packet: %v", err)
		}

		switch h.packetType {
		case rtcpReceiverReportType:
			if 4*h.length != 4+h.count*rtcpReportSize {
				return errors.Errorf("invalid Receiver Report: length = %d, count = %d", h.length, h.count)
			}
			p := new(RTCPReceiverReport)
			p.readFrom(r, h.count)
			cp.packets = append(cp.packets, p)
		default:
			log.Info("Skipping unimplemented RTCP packet type: %d", h.packetType)
			r.ReadSlice(4 * h.length)
		}
	}

	return nil
}

// Report block for sender and receiver reports. See
// https://tools.ietf.org/html/rfc3550#section-6.4.1.
type RTCPReport struct {
	// The source that this report refers to.
	Source uint32

	// Fraction of packets lost since last report for this source.
	FractionLost float32

	// Total packets lost from this source for the entire session.
	TotalLost int

	// Extended sequence number of last packet received from this source.
	LastReceived uint32

	// Interarrival jitter, measured in timestamp units.
	Jitter uint32

	// Truncated NTP timestamp of most recent Sender Report from this source.
	LastSenderReportTimestamp uint32

	// Time in 1/65536 seconds since the most recent Sender Report from this
	// source (or 0, if no SR has been received).
	LastSenderReportDelay uint32
}

func (report RTCPReport) writeTo(w *packet.Writer) {
	w.WriteUint32(uint32(report.Source))
	w.WriteByte(byte(report.FractionLost * 256))
	w.WriteUint24(uint32(report.TotalLost))
	w.WriteUint32(report.LastReceived)
	w.WriteUint32(report.Jitter)
	w.WriteUint32(report.LastSenderReportTimestamp)
	w.WriteUint32(report.LastSenderReportDelay)
}

func (report *RTCPReport) readFrom(r *packet.Reader) {
	report.Source = r.ReadUint32()
	report.FractionLost = float32(r.ReadByte()) / 256
	report.TotalLost = int(r.ReadUint24())
	report.LastReceived = r.ReadUint32()
	report.Jitter = r.ReadUint32()
	report.LastSenderReportTimestamp = r.ReadUint32()
	report.LastSenderReportDelay = r.ReadUint32()
}

type RTCPReceiverReport struct {
	Sender  uint32
	Reports []RTCPReport
}

func (p *RTCPReceiverReport) writeTo(w *packet.Writer) error {
	h := rtcpHeader{
		count:      len(p.Reports),
		packetType: rtcpReceiverReportType,
		length:     1 + 6*len(p.Reports),
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return errors.Errorf("short ReceiverReport: %v", err)
	}
	w.WriteUint32(uint32(p.Sender))
	for i := range p.Reports {
		p.Reports[i].writeTo(w)
	}
	return nil
}

func (p *RTCPReceiverReport) readFrom(r *packet.Reader, count int) {
	p.Sender = r.ReadUint32()
	var report RTCPReport
	for i := 0; i < count; i++ {
		report.readFrom(r)
		p.Reports = append(p.Reports, report)
	}
}
