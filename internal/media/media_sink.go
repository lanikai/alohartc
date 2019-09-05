//////////////////////////////////////////////////////////////////////////////
//
// Media sink interfaces and universal implementations
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package media

import (
	"io"
)

// MediaSink is the interface for media sinks (e.g. speaker, display)
type MediaSink interface {
	io.Closer
	io.Writer
}
