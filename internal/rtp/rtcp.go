package rtp

import (
	"net"
	"sync"

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
	padding    bool // unused
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
	if err := w.CheckCapacity(rtcpHeaderSize); err != nil {
		return errors.Errorf("insufficient buffer for RTCP header: %v", err)
	}
	w.WriteByte(joinByte215(rtpVersion, h.padding, byte(h.count)))
	w.WriteByte(h.packetType)
	w.WriteUint16(uint16(h.length))
	return nil
}

type rtcpPacket interface {
	// Serialize.
	writeTo(w *packet.Writer) error

	// Deserialize.
	readFrom(r *packet.Reader, h *rtcpHeader) error
}

// A compound RTCP packet consists of one or more RTCP packets, each aligned to
// 4-byte boundaries and concatenated into a single datagram.
// See https://tools.ietf.org/html/rfc3550#section-6.1.
type rtcpCompoundPacket struct {
	packets []rtcpPacket
}

func (cp *rtcpCompoundPacket) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if err := r.CheckRemaining(rtcpHeaderSize); err != nil {
		return errors.Errorf("short buffer: %v", err)
	}

	if cp.packets != nil {
		cp.packets = cp.packets[:0]
	}

	var p rtcpPacket
	for {
		switch h.packetType {
		case rtcpReceiverReportType:
			p = new(rtcpReceiverReport)
		case rtcpSourceDescriptionType:
			p = new(rtcpSourceDescription)
		default:
			log.Debug("Skipping unimplemented RTCP packet type: %d", h.packetType)
			r.Skip(4 * h.length)
			continue
		}

		if err := p.readFrom(r, h); err != nil {
			return err
		}
		cp.packets = append(cp.packets, p)

		if r.Remaining() == 0 {
			break
		}

		// Read next packet header.
		if err := h.readFrom(r); err != nil {
			return err
		}
		if err := r.CheckRemaining(4 * h.length); err != nil {
			return errors.Errorf("short RTCP packet: %v", err)
		}
	}

	return nil
}

func (cp *rtcpCompoundPacket) writeTo(w *packet.Writer) error {
	for _, p := range cp.packets {
		if err := p.writeTo(w); err != nil {
			return err
		}
	}
	return nil
}

// Report block for sender and receiver reports.
// See https://tools.ietf.org/html/rfc3550#section-6.4.1
type rtcpReport struct {
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

func (report rtcpReport) writeTo(w *packet.Writer) {
	w.WriteUint32(uint32(report.Source))
	w.WriteByte(byte(report.FractionLost * 256))
	w.WriteUint24(uint32(report.TotalLost))
	w.WriteUint32(report.LastReceived)
	w.WriteUint32(report.Jitter)
	w.WriteUint32(report.LastSenderReportTimestamp)
	w.WriteUint32(report.LastSenderReportDelay)
}

func (report *rtcpReport) readFrom(r *packet.Reader) {
	report.Source = r.ReadUint32()
	report.FractionLost = float32(r.ReadByte()) / 256
	report.TotalLost = int(r.ReadUint24())
	report.LastReceived = r.ReadUint32()
	report.Jitter = r.ReadUint32()
	report.LastSenderReportTimestamp = r.ReadUint32()
	report.LastSenderReportDelay = r.ReadUint32()
}

// Receiver Report (RR) RTCP packet.
// See https://tools.ietf.org/html/rfc3550#section-6.4.2
type rtcpReceiverReport struct {
	receiver uint32 // SSRC of receiver who sent the report
	reports  []rtcpReport
}

func (p *rtcpReceiverReport) writeTo(w *packet.Writer) error {
	h := rtcpHeader{
		packetType: rtcpReceiverReportType,
		count:      len(p.reports),
		length:     (4 + len(p.reports)*rtcpReportSize) / 4,
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return errors.Errorf("insufficient buffer for ReceiverReport: %v", err)
	}
	w.WriteUint32(uint32(p.receiver))
	for i := range p.reports {
		p.reports[i].writeTo(w)
	}
	return nil
}

func (p *rtcpReceiverReport) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if 4*h.length != 4+h.count*rtcpReportSize {
		return errors.Errorf("invalid Receiver Report: length = %d, count = %d", h.length, h.count)
	}

	p.receiver = r.ReadUint32()
	var report rtcpReport
	for i := 0; i < h.count; i++ {
		report.readFrom(r)
		p.reports = append(p.reports, report)
	}
	return nil
}

// Source Description (SDES) RTCP packet.
// See https://tools.ietf.org/html/rfc3550#section-6.5
type rtcpSourceDescription struct {
	ssrc  uint32
	cname string
	// TODO: Are any other SDES items needed?
}

func (sdes *rtcpSourceDescription) writeTo(w *packet.Writer) error {
	items := []sdesItem{
		{sdesItemCNAME, sdes.cname},
		{sdesItemEnd, ""},
	}
	totalSize := 0
	for _, item := range items {
		totalSize += item.size()
	}

	h := rtcpHeader{
		packetType: rtcpSourceDescriptionType,
		count:      1,
		length:     1 + (totalSize+3)/4,
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return errors.Errorf("insufficient buffer for SDES packet: %v", err)
	}

	w.WriteUint32(sdes.ssrc)
	for _, item := range items {
		item.writeTo(w)
	}

	return nil
}

func (sdes *rtcpSourceDescription) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if h.count != 1 || h.length < 1 {
		return errors.Errorf("invalid SDES packet header: %#v", h)
	}
	sdes.ssrc = r.ReadUint32()

	var item sdesItem
	for r.Remaining() > 0 {
		item.readFrom(r)
		switch item.what {
		case sdesItemEnd:
			return nil
		case sdesItemCNAME:
			sdes.cname = item.text
		default:
			log.Trace(4, "Ignoring unimplemented SDES item type: %d", item.what)
		}
	}
	return nil
}

const (
	sdesItemEnd   = 0
	sdesItemCNAME = 1
)

type sdesItem struct {
	what byte
	text string
}

// Number of bytes occupied by this SDES item.
func (item *sdesItem) size() int {
	if item.what == sdesItemEnd {
		return 1
	}
	return 2 + len(item.text)
}

func (item *sdesItem) writeTo(w *packet.Writer) {
	w.WriteByte(item.what)
	if item.what == sdesItemEnd {
		w.Align(4)
	} else {
		w.WriteByte(uint8(len(item.text)))
		w.WriteString(item.text)
	}
}

func (item *sdesItem) readFrom(r *packet.Reader) {
	item.what = r.ReadByte()
	if item.what == sdesItemEnd {
		// Discard zeros up to the next 32-bit (i.e. 4-byte) boundary.
		r.Align(4)
	} else {
		length := int(r.ReadByte())
		item.text = r.ReadString(length)
	}
}

// rtcpWriter maintains state necessary for sending RTCP packets.
type rtcpWriter struct {
	conn net.Conn
	ssrc uint32

	// Number of RTCP packets sent.
	count uint64

	// Total number of RTCP bytes sent.
	totalBytes uint64

	// Reusable buffer for serializing packets.
	buf []byte

	// SRTP cryptographic context.
	crypto *cryptoContext

	// Prevent simultaneous writes from multiple goroutines.
	sync.Mutex
}

func newRTCPWriter(conn net.Conn, ssrc uint32, crypto *cryptoContext) *rtcpWriter {
	w := new(rtcpWriter)
	w.conn = conn
	w.ssrc = ssrc
	w.buf = make([]byte, 1500) // TODO: Determine from MTU
	w.crypto = crypto          // By value so that we have our own copy
	return w
}

func (w *rtcpWriter) index() uint64 {
	return w.count
}

func (w *rtcpWriter) writePacket(p rtcpPacket) error {
	w.Lock()
	defer w.Unlock()

	b := packet.NewWriter(w.buf)
	if err := p.writeTo(b); err != nil {
		return err
	}

	index := w.index()
	if w.crypto != nil {
		if err := w.crypto.encryptAndSignRTCP(b, index); err != nil {
			return err
		}
	}

	w.count += 1
	w.totalBytes += uint64(b.Length())

	_, err := w.conn.Write(b.Bytes())
	return err
}

func (w *rtcpWriter) writePackets(p ...rtcpPacket) error {
	switch len(p) {
	case 0:
		return nil
	case 1:
		return w.writePacket(p[0])
	default:
		cp := &rtcpCompoundPacket{p}
		return w.writePacket(cp)
	}
}
