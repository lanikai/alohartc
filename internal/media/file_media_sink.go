//////////////////////////////////////////////////////////////////////////////
//
// File media sink
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package media

import (
	"os"
)

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
