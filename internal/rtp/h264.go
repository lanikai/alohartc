package rtp

import (
	"context"
	"io"

	"github.com/lanikai/alohartc/internal/media"
	"github.com/lanikai/alohartc/internal/media/h264"
	"github.com/lanikai/alohartc/internal/packet"
)

// RTP packetization of H.264 video streams.
// See [RFC 6184](https://tools.ietf.org/html/rfc6184).

func (s *Stream) SendVideo(ctx context.Context, payloadType byte, localVideo media.VideoSource) error {
	initialTimestamp := uint32(0) // TODO: randomize timestamp

	w := h264Writer{
		rtpWriter:   s.rtpOut,
		payloadType: payloadType,
		timestamp:   initialTimestamp,
	}

	videoCh := localVideo.AddReceiver(4)
	defer localVideo.RemoveReceiver(videoCh)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case buf, more := <-videoCh:
			if !more {
				log.Debug("Received EOF from video source: %v", localVideo)
				return io.EOF
			}
			if err := w.consume(buf); err != nil {
				return err
			}
		}
		// TODO: Sender reports, RTCP feedback, etc.
	}
}

const (
	// NAL unit types. See https://tools.ietf.org/html/rfc6184#section-5.2
	naluTypeSEI    = 6
	naluTypeSPS    = 7
	naluTypePPS    = 8
	naluTypeSTAP_A = 24
	naluTypeFU_A   = 28
)

// TODO: Generalize h264Writer.
type videoWriter interface {
	consume(buf *media.SharedBuffer) error
}

type h264Writer struct {
	*rtpWriter

	payloadType byte
	timestamp   uint32

	// Accumulated STAP-A packet. This is initialized when a SPS or PPS is
	// encountered, and saved until the next coded picture needs to be sent.
	stap []byte
}

func (w *h264Writer) consume(buf *media.SharedBuffer) error {
	defer buf.Release()

	nalu := h264.NALU(buf.Bytes())
	switch nalu.Type() {
	case naluTypeSEI, naluTypeSPS, naluTypePPS:
		// Merge consecutive SEI/SPS/PPS into a single STAP-A packet.
		w.appendSTAP(nalu)
		return nil
	default:
		return w.packetize(nalu)
	}
}

// See https://tools.ietf.org/html/rfc6184#section-5.7.1
func (w *h264Writer) appendSTAP(nalu h264.NALU) {
	n := len(nalu)
	if len(w.stap) == 0 {
		// Initialize NALU of type STAP-A, with F and NRI set to 0.
		w.stap = append(w.stap, naluTypeSTAP_A)
	}
	w.stap = append(w.stap, byte(n>>8), byte(n))
	w.stap = append(w.stap, nalu...)

	// STAP-A forbidden bit is bitwise-OR of all forbidden bits.
	w.stap[0] |= nalu[0] & 0x80

	// STAP-A NRI value is maximum of all NRI values.
	nri := nalu[0] & 0x60
	stapNRI := w.stap[0] & 0x60
	if nri > stapNRI {
		w.stap[0] = (w.stap[0] &^ 0x60) | nri
	}
}

func (w *h264Writer) advanceTimestamp() {
	// TODO: Use framerate from video source
	w.timestamp += 3000
}

func (w *h264Writer) packetize(nalu h264.NALU) error {
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
	typ := nalu.Type()
	p := packet.NewWriterSize(maxSize) // TODO: sync.Pool
	for i := 1; i < len(nalu); i += maxSize - 2 {
		tail := i + maxSize - 2
		if tail >= len(nalu) {
			tail = len(nalu)
			end = 0x40
		}

		p.WriteByte(indicator)         // FU indicator
		p.WriteByte(start | end | typ) // FU header
		p.WriteSlice(nalu[i:tail])

		if err := w.writePacket(w.payloadType, end != 0, w.timestamp, p.Bytes()); err != nil {
			return err
		}

		p.Reset()
		start = 0
	}
	return nil
}
