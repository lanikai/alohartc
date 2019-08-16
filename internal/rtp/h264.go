package rtp

import (
	"bytes"
	"io"
	"time"

	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/packet"
)

// RTP packetization of H.264 video streams.
// See [RFC 6184](https://tools.ietf.org/html/rfc6184).

const (
	// NAL unit types. See https://tools.ietf.org/html/rfc6184#section-5.2
	naluTypeSEI    = 6
	naluTypeSPS    = 7
	naluTypePPS    = 8
	naluTypeSTAP_A = 24
	naluTypeFU_A   = 28
)

func (s *Stream) SendVideo(quit <-chan struct{}, payloadType byte, src media.VideoSource) error {
	initialTimestamp := uint32(0) // TODO: randomize timestamp

	w := h264Writer{
		rtpWriter:   s.rtpOut,
		payloadType: payloadType,
		timestamp:   initialTimestamp,
	}

	r := src.AddReceiver(16)
	defer src.RemoveReceiver(r)

	for {
		select {
		case <-quit:
			return nil
		case buf, more := <-r.Buffers():
			if !more {
				log.Debug("SendVideo %d stopping: %v", payloadType, r.Err())
				return r.Err()
			}
			if err := w.consume(buf); err != nil {
				return err
			}
		}
		// TODO: Sender reports, RTCP feedback, etc.
	}
}

func (s *Stream) ReceiveVideo(quit <-chan struct{}, consume func(buf *packet.SharedBuffer) error) error {
	r := h264Reader{
		rtpReader: s.rtpIn,
		ch:        make(chan *packet.SharedBuffer, 4),
	}
	s.rtpIn.handler = r.handlePacket

	receiverReportTicker := time.NewTicker(2 * time.Second)
	defer receiverReportTicker.Stop()

	for {
		select {
		case <-quit:
			return nil
		case buf, more := <-r.ch:
			if !more {
				return io.EOF
			}

			if err := consume(buf); err != nil {
				return err
			}
		case <-receiverReportTicker.C:
			log.Debug("sending Receiver Report for remote SSRC %02x", s.RemoteSSRC)
			s.sendReceiverReport()
		}
	}
}

type h264Writer struct {
	*rtpWriter

	payloadType byte
	timestamp   uint32

	// Accumulated STAP-A packet. This is initialized when a SPS or PPS is
	// encountered, and saved until the next coded picture needs to be sent.
	stap []byte
}

func (w *h264Writer) consume(buf *packet.SharedBuffer) error {
	defer buf.Release()

	nalu := buf.Bytes()
	naluType := nalu[0] & 0x1f
	switch naluType {
	case naluTypeSEI, naluTypeSPS, naluTypePPS:
		// Merge consecutive SEI/SPS/PPS into a single STAP-A packet.
		w.stap = appendSTAP(w.stap, nalu)
		return nil
	default:
		return w.packetize(nalu)
	}
}

func (w *h264Writer) advanceTimestamp() {
	// TODO: Use framerate from video source
	w.timestamp += 3000
}

func (w *h264Writer) packetize(nalu []byte) error {
	// First send STAP-A packet, if present.
	if len(w.stap) > 0 {
		if err := w.writePacket(w.payloadType, false, w.timestamp, w.stap); err != nil {
			return err
		}
		w.stap = w.stap[:0]
	}

	defer w.advanceTimestamp()

	// Maximum payload size.
	// TODO: Get this from the rtpWriter.
	maxSize := 1280

	// If it fits, send the NALU as a single RTP packet.
	// See https://tools.ietf.org/html/rfc6184#section-5.6
	if len(nalu) < maxSize {
		return w.writePacket(w.payloadType, true, w.timestamp, nalu)
	}

	// Otherwise, fragment the NALU into multiple FU-A packets.
	// See https://tools.ietf.org/html/rfc6184#section-5.8
	indicator := nalu[0]&0xe0 | naluTypeFU_A
	start := byte(0x80)
	end := byte(0)
	naluType := nalu[0] & 0x1f
	p := packet.NewWriterSize(maxSize) // TODO: sync.Pool
	for i := 1; i < len(nalu); i += maxSize - 2 {
		tail := i + maxSize - 2
		if tail >= len(nalu) {
			tail = len(nalu)
			end = 0x40
		}

		p.Reset()
		p.WriteByte(indicator)              // FU indicator
		p.WriteByte(start | end | naluType) // FU header
		p.WriteSlice(nalu[i:tail])

		if err := w.writePacket(w.payloadType, end != 0, w.timestamp, p.Bytes()); err != nil {
			return err
		}

		start = 0
	}
	return nil
}

type h264Reader struct {
	*rtpReader

	// Channel for received NAL units.
	ch chan *packet.SharedBuffer

	// Buffer for assembling FU-A packets into a complete NALU.
	buf *bytes.Buffer
}

func copyBytes(buf []byte) []byte {
	return append([]byte(nil), buf...)
}

func (r *h264Reader) handlePacket(hdr rtpHeader, payload []byte) error {
	log.Trace(4, "Received RTP payload: %d", len(payload))

	// Assemble RTP packets into full NAL units.
	naluType := payload[0] & 0x1f
	switch naluType {
	case naluTypeSTAP_A:
		// STAP-A packet potentially contains SEI, SPS, and PPS.
		payload = copyBytes(payload)
		nalus, err := splitSTAP(payload)
		if err != nil {
			return err
		}
		for _, nalu := range nalus {
			r.ch <- packet.NewSharedBuffer(nalu, 1, nil)
		}
	case naluTypeFU_A:
		// Reassemble a sequence of FU-A packets.
		// See https://tools.ietf.org/html/rfc6184#section-5.8
		indicator := payload[0]
		header := payload[1]
		start := header & 0x80
		end := header & 0x40
		if start != 0 {
			r.buf = new(bytes.Buffer) // TODO: sync.Pool
			fnri := indicator & 0xe0
			naluType := header & 0x1f
			r.buf.WriteByte(fnri | naluType)
		} else if r.buf == nil {
			// Wait for the start of the next NALU.
			break
		}
		r.buf.Write(payload[2:])
		if end != 0 {
			r.ch <- packet.NewSharedBuffer(r.buf.Bytes(), 1, nil)
			r.buf = nil
		}
	default:
		// Payload is a single NALU.
		payload = copyBytes(payload)
		r.ch <- packet.NewSharedBuffer(payload, 1, nil)
	}
	return nil
}

// See https://tools.ietf.org/html/rfc6184#section-5.7.1
func appendSTAP(stap, nalu []byte) []byte {
	if len(stap) == 0 {
		// Initialize NALU of type STAP-A, with F and NRI set to 0.
		stap = append(stap, naluTypeSTAP_A)
	}

	n := len(nalu)
	stap = append(stap, byte(n>>8), byte(n))
	stap = append(stap, nalu...)

	// STAP-A forbidden bit is bitwise-OR of all forbidden bits.
	stap[0] |= nalu[0] & 0x80

	// STAP-A NRI value is maximum of all NRI values.
	nri := nalu[0] & 0x60
	stapNRI := stap[0] & 0x60
	if nri > stapNRI {
		stap[0] = (stap[0] &^ 0x60) | nri
	}

	return stap
}

// Split a STAP-A packet into individual NAL units.
func splitSTAP(buf []byte) ([][]byte, error) {
	var nalus [][]byte
	p := packet.NewReader(buf)
	p.Skip(1)
	for p.Remaining() > 0 {
		if err := p.CheckRemaining(2); err != nil {
			return nil, err
		}
		n := p.ReadUint16()
		if err := p.CheckRemaining(int(n)); err != nil {
			return nil, err
		}
		nalus = append(nalus, p.ReadSlice(int(n)))
	}
	return nalus, nil
}
