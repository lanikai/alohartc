package rtp

import (
	"net"
	"sync"

	errors "golang.org/x/xerrors"

	"github.com/lanikai/alohartc/internal/packet"
)

// RTP Data Transfer Protocol, as defined in RFC 3550 Section 5.

// An RTP packet consists of a fixed 12-byte header, zero or more 32-bit CSRC
// identifiers, followed by the payload itself.
// See https://tools.ietf.org/html/rfc3550#section-5.1
//    0                   1                   2                   3
//    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//   |V=2|P|X|  CC   |M|     PT      |       sequence number         |
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//   |                           timestamp                           |
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//   |           synchronization source (SSRC) identifier            |
//   +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
//   |            contributing source (CSRC) identifiers             |
//   |                             ....                              |
//   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
type rtpHeader struct {
	padding     bool // unused
	extension   bool // unused
	marker      bool
	payloadType byte
	sequence    uint16
	timestamp   uint32
	ssrc        uint32
	csrc        []uint32 // unused
}

func (h *rtpHeader) length() int {
	return rtpHeaderSize + 4*len(h.csrc)
}

const (
	rtpHeaderSize = 12
)

func (h *rtpHeader) writeTo(w *packet.Writer) {
	w.WriteByte(joinByte2114(rtpVersion, h.padding, h.extension, byte(len(h.csrc))))
	w.WriteByte(joinByte17(h.marker, h.payloadType))
	w.WriteUint16(h.sequence)
	w.WriteUint32(h.timestamp)
	w.WriteUint32(h.ssrc)
	for i := range h.csrc {
		w.WriteUint32(h.csrc[i])
	}

	// TODO: Padding.
}

func (h *rtpHeader) readFrom(r *packet.Reader) error {
	if err := r.CheckRemaining(rtpHeaderSize); err != nil {
		return errors.Errorf("short buffer: %v", err)
	}

	var version, csrcCount byte
	version, h.padding, h.extension, csrcCount = splitByte2114(r.ReadByte())
	if version != rtpVersion {
		return errBadVersion(version)
	}
	if err := r.CheckRemaining(4 * int(csrcCount)); err != nil {
		return errors.Errorf("short buffer: %v", err)
	}
	h.marker, h.payloadType = splitByte17(r.ReadByte())
	h.sequence = r.ReadUint16()
	h.timestamp = r.ReadUint32()
	h.ssrc = r.ReadUint32()
	h.csrc = nil
	for i := 0; i < int(csrcCount); i++ {
		h.csrc = append(h.csrc, r.ReadUint32())
	}

	return nil
}

// rtpWriter maintains state necessary for sending RTP data packets.
type rtpWriter struct {
	conn net.Conn
	ssrc uint32

	// Initial sequence number. The current sequence number is computed from
	// sequenceStart and count.
	sequenceStart uint16

	// Number of RTP packets sent.
	count uint64

	// Total number of payload bytes sent.
	totalBytes uint64

	// Buffer used for serializing packets.
	buf *packet.Writer

	// SRTP cryptographic context.
	crypto cryptoContext

	sync.Mutex
}

func newRTPWriter(conn net.Conn, ssrc uint32, crypto *cryptoContext) *rtpWriter {
	w := new(rtpWriter)
	w.conn = conn
	w.ssrc = ssrc
	w.sequenceStart = 1                // TODO: Randomize initial sequence number.
	w.buf = packet.NewWriterSize(1280) // TODO: Determine from MTU
	w.crypto = *crypto                 // By value so that we have our own copy
	return w
}

// Send a single RTP packet.
func (w *rtpWriter) writePacket(payloadType byte, marker bool, timestamp uint32, payload []byte) error {
	w.Lock()
	defer w.Unlock()

	p := w.buf
	p.Reset()

	index := w.index()
	hdr := rtpHeader{
		marker:      marker,
		payloadType: payloadType,
		sequence:    uint16(index),
		timestamp:   timestamp,
		ssrc:        w.ssrc,
	}
	hdr.writeTo(p)

	if err := p.WriteSlice(payload); err != nil {
		return err
	}

	if err := w.crypto.encryptAndSignRTP(p, &hdr, index); err != nil {
		return err
	}

	w.count += 1
	w.totalBytes += uint64(len(payload))

	_, err := w.conn.Write(p.Bytes())
	return err
}

// Compute the RTP packet index, also known as the extended sequence number.
// Equivalent to rolloverCounter*2^16 + sequenceNumber (i.e. ROC || SEQ).
func (w *rtpWriter) index() uint64 {
	return w.count + uint64(w.sequenceStart)
}

// Compute the current sequence number.
func (w *rtpWriter) sequenceNumber() uint16 {
	return uint16(w.index())
}

// Compute the rollover counter, which starts at 0 and increases by 1 every time
// the 16-bit sequence number rolls over.
func (w *rtpWriter) rolloverCounter() uint32 {
	return uint32(w.index() / 65536)
}
