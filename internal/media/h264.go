package media

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

// Raw H.264 source with NALUs separated by Annex B start codes.
type h264Reader struct {
	in      io.ReadCloser
	buffer  []byte
	scanner *bufio.Scanner
}

const (
	naluBufferInitialSize = 16 * 1024
	naluBufferMaximumSize = 1024 * 1024
)

func NewH264Reader(in io.ReadCloser) H264Source {
	buffer := make([]byte, naluBufferInitialSize)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(buffer, naluBufferMaximumSize)
	scanner.Split(splitNALU)
	return &h264Reader{
		in:      in,
		buffer:  buffer,
		scanner: scanner,
	}
}

func (r *h264Reader) PayloadType() string {
	return "H264/90000"
}

func (r *h264Reader) ReadNALU() (nalu []byte, err error) {
	if r.scanner.Scan() {
		// TODO: Do we need to make a copy of this?
		nalu = r.scanner.Bytes()
	} else {
		err = r.scanner.Err()
	}
	return
}

func (r *h264Reader) Close() error {
	return r.in.Close()
}

var h264StartCode = []byte{0, 0, 1}

// Splits NAL units on H.264 Annex B start codes.
func splitNALU(data []byte, atEOF bool) (advance int, nalu []byte, err error) {
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

func openH264(filename string) (src Source, err error) {
	f, err := os.Open(filename)
	if err == nil {
		src = NewH264Reader(f)
	}
	return
}

func init() {
	RegisterSourceType("h264", openH264)
}
