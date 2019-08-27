package rtp

import (
	"io"
	"math/rand"
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
	out  io.Writer
	ssrc uint32

	// Initial sequence number. The current sequence number is computed from
	// sequenceStart and count.
	sequenceStart uint16

	// Number of RTP packets sent.
	count uint64

	// Total number of payload bytes sent.
	totalBytes uint64

	// Buffer used for serializing packets.
	buf []byte

	// SRTP cryptographic context.
	crypto *cryptoContext

	// Prevent simultaneous writes from multiple goroutines.
	sync.Mutex
}

func newRTPWriter(out io.Writer, ssrc uint32, crypto *cryptoContext) *rtpWriter {
	w := new(rtpWriter)
	w.out = out
	w.ssrc = ssrc
	w.sequenceStart = uint16(rand.Uint32())
	w.buf = make([]byte, 1500) // TODO: Determine from MTU
	w.crypto = crypto
	return w
}

// Send a single RTP packet to the remote peer.
func (w *rtpWriter) writePacket(payloadType byte, marker bool, timestamp uint32, payload []byte) error {
	w.Lock()
	defer w.Unlock()

	index := w.index()
	hdr := rtpHeader{
		marker:      marker,
		payloadType: payloadType,
		sequence:    uint16(index),
		timestamp:   timestamp,
		ssrc:        w.ssrc,
	}

	p := packet.NewWriter(w.buf)
	hdr.writeTo(p)

	if err := p.WriteSlice(payload); err != nil {
		return err
	}

	if w.crypto != nil {
		if err := w.crypto.encryptAndSignRTP(p, &hdr, index); err != nil {
			return err
		}
	}

	w.count += 1
	w.totalBytes += uint64(len(payload))

	_, err := w.out.Write(p.Bytes())
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
	return uint32(w.index() >> 16)
}

// rtpReader maintains state necessary for receiving RTP data packets.
type rtpReader struct {
	ssrc uint32

	// Most recent observed sequence number.
	lastSequence uint16

	// Estimate of the sender's RTP packet index, based on the most recent
	// observed sequence number and the number of times it has rolled over.
	lastIndex uint64

	// Number of RTP packets received.
	count uint64

	// Total number of payload bytes received.
	totalBytes uint64

	// SRTP cryptographic context.
	crypto *cryptoContext

	// Callback for RTP packets. This function should return quickly to avoid
	// blocking the RTP read loop. If it needs the payload bytes for longer than
	// the lifetime of the function call, it *must* make a copy.
	handler func(hdr rtpHeader, payload []byte) error
}

func newRTPReader(ssrc uint32, crypto *cryptoContext) *rtpReader {
	r := new(rtpReader)
	r.ssrc = ssrc
	r.crypto = crypto
	return r
}

// Read and process a single RTP packet. buf contains the serialized packet,
// which will be decrypted in place.
func (r *rtpReader) readPacket(buf []byte) error {
	p := packet.NewReader(buf)
	var hdr rtpHeader
	if err := hdr.readFrom(p); err != nil {
		return err
	}

	index := r.updateIndex(hdr.sequence)

	var payload []byte
	if r.crypto != nil {
		var err error
		if payload, err = r.crypto.verifyAndDecryptRTP(buf, &hdr, index); err != nil {
			return err
		}
	} else {
		payload = buf[hdr.length():]
	}

	r.count += 1
	r.totalBytes += uint64(len(payload))

	if r.handler == nil {
		log.Warn("received RTP packet, but no handler registered")
		return nil
	}
	return r.handler(hdr, payload)
}

// Update the rollover counter (ROC) and sequence number (SEQ), which we combine
// into a single 48-bit index variable. Return the index corresponding to the
// provided sequence number.
// See https://tools.ietf.org/html/rfc3711#section-3.3.1
func (r *rtpReader) updateIndex(sequence uint16) uint64 {
	if r.lastIndex == 0 {
		// Initialize ROC to 0, so index = SEQ.
		r.lastSequence = sequence
		r.lastIndex = uint64(sequence)
		return r.lastIndex
	}

	// If either sequence or lastSequence is close to 2^16, and the other is
	// close to 0, then correct for rollover.
	delta := int64(sequence) - int64(r.lastSequence)
	if delta > 32768 {
		delta -= 65536
	} else if delta <= -32768 {
		delta += 65536
	}
	if delta > 4096 {
		log.Debug("large RTP sequence number delta: %d -> %d", r.lastSequence, sequence)
	}

	index := uint64(int64(r.lastIndex) + delta)
	if index > r.lastIndex {
		r.lastIndex = index
		r.lastSequence = sequence
	}
	return index
}
