//////////////////////////////////////////////////////////////////////////////
//
// Config contains configuration data for PeerConnection
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"github.com/lanikai/alohartc/internal/media"
)

type Config struct {
	AudioSource media.AudioSource
	VideoSource media.VideoSource
	AudioSink   media.AudioSink

	// Set of network interfaces to use for candidate discovery
	Interfaces map[string]bool
}
