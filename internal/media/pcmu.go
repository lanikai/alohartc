//////////////////////////////////////////////////////////////////////////////
//
// PCM μ-law (ITU-T G.711) audio codec. This codec supports 8 kHz audio only.
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package media

import (
	"encoding/binary"
)

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
