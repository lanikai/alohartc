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
	"os"
)

// MediaSink is the interface for media sinks (e.g. speaker, display)
type MediaSink interface {
	io.Closer
	io.Writer
}

// AudioSink is the interface for audio sinks (e.g. speaker)
type AudioSink interface {
	MediaSink

	// Configure audio sink sample rate, number of channels, and sample format
	Configure(rate int, channels int, format int) error
}

// VideoSink is the stub interface for video sinks (e.g. display)
type VideoSink interface {
	MediaSink
}

// FileMediaSink is a generic file writer, useful for testing or writing
// audio to a pipe
type FileMediaSink struct {
	file *os.File
}

func NewFileMediaSink(filename string) (*FileMediaSink, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return &FileMediaSink{file: f}, nil
}

// Close file sink
func (s *FileMediaSink) Close() error {
	return s.file.Close()
}

// Configure file sink (to meet interface; no actual configuration required)
func (s *FileMediaSink) Configure(rate, channels, format int) error {
	return nil
}

// Write buffer to file
func (s *FileMediaSink) Write(p []byte) (int, error) {
	return s.file.Write(p)
}
