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

package media

import "io"

// Encoder is the interface for audio and video encoders
type Encoder interface {
	io.Closer

	Encode([]byte) ([]byte, error)
}
