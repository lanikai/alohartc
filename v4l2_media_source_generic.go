//////////////////////////////////////////////////////////////////////////////
//
// Stub V4L2MediaSource implementation to enable compilation on non-linux OSes
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

// +build !linux

package alohartc

import (
	"errors"
	"io"
)

type V4L2MediaSource struct {
	io.Closer
	MediaSource
}

// NewV4L2MediaSource opens a video4linux2 device with the specified settings
func NewV4L2MediaSource(
	filename string,
	width, height, bitrate uint,
	hflip, vflip bool,
) (*V4L2MediaSource, error) {
	return nil, errors.New("not supported on this operating system")
}

// Close media source
func (ms *V4L2MediaSource) Close() error {
	return errors.New("not supported on this operating system")
}

func (ms *V4L2MediaSource) GetTrack() Track {
	return nil
}

func (ms *V4L2MediaSource) CloseTrack(Track) {
	// nop
}
