//////////////////////////////////////////////////////////////////////////////
//
// Media codecs
//
// * Opus
// * PCM μ-law (ITU-T G.711)
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

// #cgo pkg-config: opus
// #include <stdlib.h>
// #include <opus/opus.h>
import "C"
import (
	"encoding/binary"
	"errors"
	"io"
	"unsafe"
)

// Encoder is the interface for audio and video encoders
type Encoder interface {
	io.Closer

	Encode([]byte) ([]byte, error)
}

// Decoder is the interface for audio and video decoders
type Decoder interface {
	io.Closer

	Decode(b []byte) ([]byte, error)
}

///////////////////////////////////  OPUS  ///////////////////////////////////

const (
	opusBytesPerSample = 2        // Only 16-bit audio supported
	opusMaxDataBytes   = 3 * 1276 // Max bytes per encoded buffer (recommended)
	opusMaxDurationMs  = 120      // Max packet duration (milliseconds)
	opusNumChannels    = 2        // WebRTC only requires stereo
	opusSampleRate     = 48000    // WebRTC only requires 48KHz samples rate
)

var (
	// Opus only supports the following frame sizes
	supportedFrameSizes = map[int]bool{
		2500 * opusSampleRate / 1000000:  true, // 2.5ms
		5000 * opusSampleRate / 1000000:  true, // 5ms
		10000 * opusSampleRate / 1000000: true, // 10ms
		20000 * opusSampleRate / 1000000: true, // 20ms
		40000 * opusSampleRate / 1000000: true, // 40ms
		60000 * opusSampleRate / 1000000: true, // 60ms
	}
)

// OpusDecoder implements the Decoder interface for Opus
// See https://tools.ietf.org/html/rfc6716
type OpusDecoder struct {
	handle    *C.struct_OpusDecoder
	inBandFEC bool
}

// NewOpusDecoder instantiates a new Opus audio codec decoder. The decoder
// is for 48KHz stereo audio.
func NewOpusDecoder(fec bool) (*OpusDecoder, error) {
	d := &OpusDecoder{inBandFEC: fec}

	// Create new Opus decoder
	var err C.int
	d.handle = C.opus_decoder_create(
		C.int(opusSampleRate),
		C.int(opusNumChannels),
		&err,
	)
	if err < 0 {
		return nil, errors.New(C.GoString(C.opus_strerror(err)))
	}

	return d, nil
}

// Decode frame of Opus audio. Use nil for b to indicate packet loss.
// Returned audio frame will be 48KHz interleaved stereo with signed
// 16-bit samples in little endian format.
func (d *OpusDecoder) Decode(b []byte) ([]byte, error) {
	var frameSize C.int

	inBandFEC := C.int(0)
	if d.inBandFEC {
		inBandFEC = C.int(1)
	}

	// Decode
	maxFrameSize := (opusMaxDurationMs * opusSampleRate / 1000)
	out := C.malloc(opusBytesPerSample * opusNumChannels * C.uint(maxFrameSize))
	defer C.free(unsafe.Pointer(out))
	if nil == b {
		frameSize = C.opus_decode(
			d.handle,
			nil,
			C.opus_int32(0),
			(*C.opus_int16)(unsafe.Pointer(out)),
			C.int(maxFrameSize),
			inBandFEC,
		)
	} else {
		in := C.CBytes(b)
		defer C.free(unsafe.Pointer(in))
		frameSize = C.opus_decode(
			d.handle,
			(*C.uchar)(in),
			C.opus_int32(len(b)),
			(*C.opus_int16)(unsafe.Pointer(out)),
			C.int(maxFrameSize),
			inBandFEC,
		)
	}
	if frameSize < 0 {
		return nil, errors.New(C.GoString(C.opus_strerror(frameSize)))
	}

	return C.GoBytes(out, (opusBytesPerSample * opusNumChannels * frameSize)), nil
}

func (d *OpusDecoder) Close() error {
	C.opus_decoder_destroy(d.handle)
	return nil
}

// OpusEncoder implements the Encoder interface for Opus
// See https://tools.ietf.org/html/rfc6716
type OpusEncoder struct {
	handle *C.struct_OpusEncoder
}

// NewOpusEncoder instantiates a new Opus audio codec encoder. The encoder
// is for 48KHz stereo audio for VoIP applications, meaning it includes
// forward error correction for expected packet loss.
func NewOpusEncoder() (*OpusEncoder, error) {
	e := &OpusEncoder{}

	// Create new Opus encoder
	var err C.int
	e.handle = C.opus_encoder_create(
		C.int(opusSampleRate),
		C.int(opusNumChannels),
		C.OPUS_APPLICATION_VOIP,
		&err,
	)
	if err < 0 {
		return nil, errors.New(C.GoString(C.opus_strerror(err)))
	}

	return e, nil
}

// Close Opus encoder
func (e *OpusEncoder) Close() error {
	C.opus_encoder_destroy(e.handle)
	return nil
}

// Encode frame of audio. Expect 48KHz interleaved stereo with signed
// 16-bit samples in little endian format.
func (e *OpusEncoder) Encode(b []byte) ([]byte, error) {
	frameSize := len(b) / opusNumChannels / opusBytesPerSample

	// Validate frame size
	if !supportedFrameSizes[frameSize] {
		return nil, errNotSupported
	}

	// Encode
	in := C.CBytes(b)
	defer C.free(unsafe.Pointer(in))
	out := C.malloc(opusMaxDataBytes)
	defer C.free(unsafe.Pointer(out))
	n := C.opus_encode(
		e.handle,
		(*C.opus_int16)(in),
		C.int(frameSize),
		(*C.uchar)(out),
		C.opus_int32(opusMaxDataBytes),
	)
	if n < 0 {
		return nil, errors.New(C.GoString(C.opus_strerror(n)))
	}

	return C.GoBytes(out, n), nil
}

///////////////////////////////////  PCMU  ///////////////////////////////////

// PCMUDecoder implements the Decoder interface for PCM μ-law
type PCMUDecoder struct {
}

// NewPCMUDecoder returns a new μ-law decoder
func NewPCMUDecoder() *PCMUDecoder {
	return &PCMUDecoder{}
}

// Decode μ-law encoded buffer b into plain audio.
// Decodes each 8-bit sample into a 14-bit signed linear audio sample,
// normalized into a 16-bit signed linear audio sample (see companding
// table). Thus, the output buffer will be twice the length of the input
// buffer.
func (d *PCMUDecoder) Decode(b []byte) ([]byte, error) {
	buffer := make([]byte, 2*len(b))
	for i, sample := range b {
		pcm := pcmuDecoderTable[sample]
		binary.LittleEndian.PutUint16(buffer[2*i:], uint16(pcm))
	}
	return buffer, nil
}

// PCMUEncoder implements the Encoder interface for PCM μ-law
type PCMUEncoder struct {
}

// NewPCMUEncoder returns a new μ-law decoder
func NewPCMUEncoder() *PCMUEncoder {
	return &PCMUEncoder{}
}

// Encode plain audio buffer b into μ-law.
// Audio samples in b are expected in 16-bit little endian format, normalized
// to use the entire 16-bit range. Only the upper 13-bits of each sample are
// used for companding.
func (e *PCMUEncoder) Encode(b []byte) ([]byte, error) {
	buffer := make([]byte, len(b)>>1)
	for i := 0; i < len(b); i += 2 {
		sample := binary.LittleEndian.Uint16(b[i:])
		buffer[i>>1] = pcmuEncoderTable[sample>>3]
	}
	return buffer, nil
}
