//////////////////////////////////////////////////////////////////////////////
//
// Media decoder interface for codecs
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package media

import "io"

// Decoder is the interface for audio and video decoders
type Decoder interface {
	io.Closer

	Decode(b []byte) ([]byte, error)
}
