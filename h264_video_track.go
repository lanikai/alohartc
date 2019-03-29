//////////////////////////////////////////////////////////////////////////////
//
// H264VideoTrack implements an H.264 specific video track
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"bufio"
	"bytes"
	"io"
)

const (
	// Maximum NAL unit size supported by the implementation
	MaxNALUnitSize = megabyte

	// Initial scanner buffer size (set to max as currently, if too small to
	// fit NAL, tail of NAL will be truncated)
	initialScanBufSize = MaxNALUnitSize
)

type H264VideoTrack struct {
	// Implements Track interface
	Track

	scanner *bufio.Scanner
}

// NewH264VideoTrack returns a new video track for the specified reader
func NewH264VideoTrack(r io.Reader) H264VideoTrack {
	scanner := bufio.NewScanner(r)

	scanner.Buffer(make([]byte, initialScanBufSize), MaxNALUnitSize)
	scanner.Split(splitNALU)

	return H264VideoTrack{
		scanner: scanner,
	}
}

// Close the video track
func (vt H264VideoTrack) Close() error {
	return nil
}

// PayloadType returns a string denoting the codec and clock rate. The Track
// interface requires the method. The string is SDP compatible.
func (vt H264VideoTrack) PayloadType() string {
	return "H264/90000"
}

// Read next packet of video track. This packet will be a STAP-A or NAL unit.
// The packet may need to be fragmented.
func (vt H264VideoTrack) Read(p []byte) (n int, err error) {
	// Blocking read and parsing
	if ok := vt.scanner.Scan(); ok {
		if nalu := vt.scanner.Bytes(); len(nalu) < 1 {
			return 0, nil
		} else {
			return copy(p, nalu), nil
		}
	}

	// Scanner returns not-ok, but empty error, on end-of-track
	if vt.scanner.Err() == nil {
		return len(vt.scanner.Bytes()), io.EOF
	}

	// Scanner returned with an error
	return len(vt.scanner.Bytes()), vt.scanner.Err()
}

// h264StartCode delineates NAL units. Code may be either 3 of 4 bytes.
var h264StartCode = []byte{0, 0, 1}

// splitNALU splits NAL units on H.264 Annex B start codes
func splitNALU(data []byte, atEOF bool) (advance int, nalu []byte, err error) {
	// Check if at end-of-track
	if atEOF {
		return 0, nil, io.EOF
	}

	// TODO: Avoid re-scanning entire data slice in the case where our last check didn't
	// find a start code. This will require storing an offset index, e.g.
	//   i := bytes.Index(data[r.offset:], ...)
	//   if no start code found {
	//       r.offset = len(data) - 2
	//   }
	i := bytes.Index(data, h264StartCode)

	switch i {
	case -1:
		// No start code found. Wait for more data.
		advance = 0
	case 0:
		// 3-byte start code (0x000001) found at data[0]. Skip these 3 bytes.
		advance = 3
	case 1:
		// 4-byte start code (0x00000001) found at data[0]. Skip these 4 bytes.
		advance = 4
	default:
		// Next start code found at index i.
		advance = i + 3
		if data[i-1] == 0x00 {
			// 4-byte start code
			nalu = data[0 : i-1]
		} else {
			// 3-byte start code
			nalu = data[0:i]
		}
	}

	return
}
