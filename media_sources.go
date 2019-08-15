//////////////////////////////////////////////////////////////////////////////
//
// Media source interfaces and universal implementations
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"io"
)

// * If possible, be able to export C interface which emulates WebRTC API
//   Can interfaces be used from C?
// * Some sources may have both audio and video (e.g. RTSP)
// * Some sources output encoded data (e.g. V4L2). Other sources output
//   data which must be encoded (e.g. ALSA soundard), V4L2 YUYV.
// * WebRTC should be able to actuate source (e.g. adjust bitrate, adjust
//   framerate, force and IDR)
// * Sources must be able to have multiple subscribers

// MediaSourcer is the interface used for providers of media, such as
// microphones and cameras.
//
// Multiple readers must be supported and each reader must receive a copy of
// source content from the time they join. How much content is buffered if
// a reader fails to read in time is left to the implementation.
type MediaSourcer interface {
	Subscriber
	io.Closer
}

const (
	// Audio sample formats
	S8    = iota // Signed 8-bit
	U8    = iota // Unsigned 8-bit
	S16LE = iota // Signed 16-bit little endian
)

// AudioSourcer is the interface that extends the basic MediaSourcer
// interface for audio sources.
type AudioSourcer interface {
	MediaSourcer

	Configure(rate, channels, format int) error
}

// VideoSourcer is the interface that extends the basic MediaSourcer
// interface for video producers.
type VideoSourcer interface {
	MediaSourcer

	// Force video source to produce an IDR rate.
	// If not supported, return errNotSupported.
	ForceIDR() error

	// SetBitrate for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetBitRate(br int) error

	// SetFramerate for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetFrameRate(fr int) error

	// SetFramesize for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetFrameSize(fs int) error
}

type RTSPAudioVideoSource struct {
	AudioSourcer
	VideoSourcer
}

func NewRTSPAudioVideoSource(url string) (*RTSPAudioVideoSource, error) {
	return nil, errNotImplemented
}

// MediaSource is the interface
type MediaSource interface {
	io.Closer

	GetTrack() Track
	CloseTrack(Track)
}
