//////////////////////////////////////////////////////////////////////////////
//
// V4L2MediaSource implements a video4linux2 API media source
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"io"

	"github.com/lanikai/alohartc/internal/v4l2"
)

type V4L2MediaSource struct {
	io.Closer
	device *v4l2.VideoReader

	tracks map[Track]*Buffer
}

// NewV4L2MediaSource opens a video4linux2 device with the specified settings
func NewV4L2MediaSource(
	filename string,
	width, height, bitrate uint,
	hflip, vflip bool,
) (*V4L2MediaSource, error) {
	// Open device
	dev, err := v4l2.Open(
		filename,
		&v4l2.Config{
			Width:                width,
			Height:               height,
			Format:               v4l2.V4L2_PIX_FMT_H264,
			RepeatSequenceHeader: true,
		},
	)
	if err != nil {
		return nil, err
	}

	// Set initial bitrate
	if err := dev.SetBitrate(bitrate); err != nil {
		return nil, err
	}

	// Flip horizontally
	if hflip {
		if err := dev.FlipHorizontal(); err != nil {
			return nil, err
		}
	}

	// Flip vertically
	if vflip {
		if err := dev.FlipVertical(); err != nil {
			return nil, err
		}
	}

	return &V4L2MediaSource{
		device: dev,
		tracks: make(map[Track]*Buffer),
	}, nil
}

// Close media source. Closes the underlying video4linux2 device.
func (ms *V4L2MediaSource) Close() error {
	return ms.device.Close()
}

// GetTrack returns a new track for the media source. Each track may be read
// by a single reader, but any number of tracks may be requested.
func (ms *V4L2MediaSource) GetTrack() Track {
	// Add a new track
	buf := NewBuffer()
	track := NewH264VideoTrack(buf)
	ms.tracks[track] = buf

	// If first track, start device and read loop
	if len(ms.tracks) == 1 {
		ms.device.Start()
		go ms.readLoop()
	}

	return track
}

// CloseTrack when the reader is done with the track
func (ms *V4L2MediaSource) CloseTrack(track Track) {
	// If last track, stop device
	if len(ms.tracks) == 1 {
		ms.device.Stop()
	}

	// Delete track
	delete(ms.tracks, track)
}

// readLoop repeatedly reads from the underlying video4linux2 device and
// writes each read frame to each reader. It exits when the device is
// closed or an unrecoverable error occurs.
func (ms *V4L2MediaSource) readLoop() {
	buf := make([]byte, 256*1024)
	for {
		if n, err := ms.device.Read(buf); err != nil {
			return
		} else {
			for _, buffer := range ms.tracks {
				buffer.Write(buf[:n])
			}
		}
	}
}
