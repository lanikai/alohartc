//////////////////////////////////////////////////////////////////////////////
//
// Media codecs (i.e. coders/decoders) such as Opus, G.711, etc.
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

// #cgo pkg-config: opus
// #include <stdlib.h>
// #include <opus/opus.h>
import "C"
import "encoding/binary"

// Encoder is the interface for audio and video encoders
type Encoder interface {
	Encode([]byte) ([]byte, error)
}

// Decoder is the interface for audio and video decoders
type Decoder interface {
	Decode(b []byte) ([]byte, error)
}

// OpusDecoder implements the Decoder interface for Opus
// See https://tools.ietf.org/html/rfc6716
type OpusDecoder struct {
}

func NewOpusDecoder() *OpusDecoder {
	return &OpusDecoder{}
}

func (d *OpusDecoder) Decode(b []byte) ([]byte, error) {
	return nil, errNotImplemented
}

// OpusEncoder implements the Encoder interface for Opus
// See https://tools.ietf.org/html/rfc6716
type OpusEncoder struct {
}

func NewOpusEncoder() *OpusEncoder {
	return &OpusEncoder{}
}

func (e *OpusEncoder) Encode(b []byte) ([]byte, error) {
	return nil, errNotImplemented
}

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
