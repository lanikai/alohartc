package rtp

import (
	errors "golang.org/x/xerrors"

	"github.com/lanikai/alohartc/internal/packet"
)

// RTP/AVPF profile for RTCP-based feedback.
// See [RFC 4585](https://tools.ietf.org/html/rfc4585).

const (
	fmtNACK = 1
	fmtPLI  = 1
	fmtREMB = 15
)

func newFeedbackPacket(packetType byte, fmt int) rtcpPacket {
	if packetType == rtcpTransportLayerFeedbackType {
		switch fmt {
		case fmtNACK:
			return new(nackFeedbackMessage)
		}
	} else if packetType == rtcpPayloadSpecificFeedbackType {
		switch fmt {
		case fmtPLI:
			return new(pliFeedbackMessage)
		case fmtREMB:
			return new(rembFeedbackMessage)
		}
	}

	log.Debug("unimplemented Feedback Message: type = %d, FMT = %d", packetType, fmt)
	return nil
}

// See https://tools.ietf.org/html/rfc4585#section-6.2.1
type nackFeedbackMessage struct {
	sender uint32 // SSRC of NACK sender
	source uint32 // SSRC of media source

	pid uint16 // packet ID (sequence number of lost packet)
	blp uint16 // bitmask of following lost packets
}

func (nack *nackFeedbackMessage) writeTo(w *packet.Writer) error {
	h := rtcpHeader{
		packetType: rtcpTransportLayerFeedbackType,
		count:      fmtNACK,
		length:     3,
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return err
	}
	w.WriteUint32(nack.sender)
	w.WriteUint32(nack.source)
	w.WriteUint16(nack.pid)
	w.WriteUint16(nack.blp)

	return nil
}

func (nack *nackFeedbackMessage) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if h.length != 3 {
		return errors.Errorf("invalid NACK Feedback Message: length = %d, ", h.length)
	}
	nack.sender = r.ReadUint32()
	nack.source = r.ReadUint32()
	nack.pid = r.ReadUint16()
	nack.blp = r.ReadUint16()
	return nil
}

func (nack *nackFeedbackMessage) getLostPackets() []uint16 {
	lost := []uint16{nack.pid}
	mask := nack.blp
	seq := nack.pid + 1
	for mask != 0 {
		if mask&0x1 == 0x1 {
			lost = append(lost, seq)
		}
		seq++
		mask >>= 1
	}
	return lost
}

func (nack *nackFeedbackMessage) setLostPackets(lost []uint16) error {
	if len(lost) == 0 {
		return errors.New("NACK Feedback Message: cannot set zero lost packets")
	}
	nack.pid = lost[0]
	nack.blp = 0
	for _, seq := range lost[1:] {
		bit := seq - nack.pid - 1
		if bit >= 16 {
			return errors.Errorf("invalid lost packets for NACK: %v", lost)
		}
		nack.blp |= (1 << bit)
	}
	return nil
}

// See https://tools.ietf.org/html/rfc4585#section-6.3.1
type pliFeedbackMessage struct {
	sender uint32 // SSRC of PLI sender
	source uint32 // SSRC of media source
}

func (pli *pliFeedbackMessage) writeTo(w *packet.Writer) error {
	h := rtcpHeader{
		packetType: rtcpPayloadSpecificFeedbackType,
		count:      fmtPLI,
		length:     2,
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return err
	}
	w.WriteUint32(pli.sender)
	w.WriteUint32(pli.source)
	return nil
}

func (pli *pliFeedbackMessage) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if h.length != 2 {
		return errors.Errorf("invalid PLI Feedback Message: length = %d, ", h.length)
	}
	pli.sender = r.ReadUint32()
	pli.source = r.ReadUint32()
	return nil
}

// See https://tools.ietf.org/html/draft-alvestrand-rmcat-remb-03#section-2.2
type rembFeedbackMessage struct {
	sender  uint32 // SSRC of REMB sender
	source  uint32 // SSRC of media source
	bitrate uint32 // Total estimated maximum available bitrate
}

// TODO [chris] incomplete
func (remb *rembFeedbackMessage) writeTo(w *packet.Writer) error {
	h := rtcpHeader{
		packetType: rtcpPayloadSpecificFeedbackType,
		count:      fmtREMB,
		length:     2,
	}
	if err := h.writeTo(w); err != nil {
		return err
	}

	if err := w.CheckCapacity(4 * h.length); err != nil {
		return err
	}
	w.WriteUint32(remb.sender)
	w.WriteUint32(remb.source)
	return nil
}

func (remb *rembFeedbackMessage) readFrom(r *packet.Reader, h *rtcpHeader) error {
	if h.length != 5 {
		return errors.Errorf("invalid REMB Feedback Message: length = %d, ", h.length)
	}
	remb.sender = r.ReadUint32()
	remb.source = r.ReadUint32()
	if "REMB" != r.ReadString(4) {
		return errors.Errorf("invalid REMB Feedback Message: invalid identifier")
	}
	numSSRC := r.ReadByte()
	em := r.ReadUint24() // bitrate exponent and mantissa
	// TODO [chris] 0 if > ~4Gbps
	remb.bitrate = (em & 0x3FFFF) << (em >> 18)
	log.Debug("%v sources, estimated bitrate: %v", numSSRC, remb.bitrate)

	return nil
}
