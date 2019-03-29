//////////////////////////////////////////////////////////////////////////////
//
// FileMediaSource implements a read-from-file media source
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"io"
	"os"
)

const (
	// Read this number of bytes from file per Read(). Should be less than
	// a typical NAL unit.
	fileMediaSourceReadSize = 4 * kilobyte
)

type pipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

type FileMediaSource struct {
	file   *os.File
	tracks map[Track]pipe
}

// NewFileMediaSource creates a file source
func NewFileMediaSource(filename string) (*FileMediaSource, error) {
	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	ms := &FileMediaSource{
		file:   file,
		tracks: make(map[Track]pipe),
	}

	go ms.readLoop()

	return ms, nil
}

// Close file media source
func (ms *FileMediaSource) Close() error {
	return ms.file.Close()
}

// GetTrack opens file source as new reader.
func (ms *FileMediaSource) GetTrack() Track {
	// Create a new pipe
	reader, writer := io.Pipe()
	p := pipe{reader, writer}

	// Create a new track
	track := NewH264VideoTrack(reader)
	ms.tracks[track] = p

	return track
}

// CloseTrack closes file source reader.
func (ms *FileMediaSource) CloseTrack(track Track) {
	if p, ok := ms.tracks[track]; ok {
		// Delete the track and close the pipe writer.
		// Reader will return io.EOF.
		delete(ms.tracks, track)
		p.writer.Close()
	}
}

// readLoop repeatedly reads from the underlying video4linux2 device and
// writes each read frame to each reader. It exits when the device is
// closed or an unrecoverable error occurs.
func (ms *FileMediaSource) readLoop() {
	buf := make([]byte, fileMediaSourceReadSize)
	for {
		if n, err := ms.file.Read(buf); err != nil {
			return
		} else {
			for _, p := range ms.tracks {
				p.writer.Write(buf[:n])
			}
		}
	}
}
