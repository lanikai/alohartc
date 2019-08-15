//////////////////////////////////////////////////////////////////////////////
//
// Config contains configuration data for PeerConnection
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

<<<<<<< HEAD
import (
	"github.com/lanikai/alohartc/internal/media"
)
=======
type Config struct {
	AudioSink   *AudioSinker
	AudioSource AudioSourcer
>>>>>>> Two-way audio functional

type Config struct {
	LocalAudio  media.AudioSource
	LocalVideo  media.VideoSource
	RemoteAudio media.AudioSink
	RemoteVideo media.VideoSink
}
