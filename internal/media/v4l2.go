// +build linux
//
// Allows streaming from a V4L2 video device (as long as it writes H.264).
// Example source spec: "h264:/dev/video0"

package media

import (
	"bytes"

	"github.com/lanikai/alohartc/internal/v4l2"
)

type v4l2Source struct {
	reader *v4l2.VideoReader

	buffer []byte
	nalus  [][]byte

	started bool
}

func (src *v4l2Source) PayloadType() string {
	return "H264/90000"
}

func (src *v4l2Source) ReadNALU() ([]byte, error) {
	if !src.started {
		if err := src.reader.Start(); err != nil {
			return nil, err
		}
		src.started = true
	}

	for len(src.nalus) == 0 {
		n, err := src.reader.Read(src.buffer)
		if err != nil {
			return nil, err
		}
		data := src.buffer[0:n]

		// The V4l2 reader may return multiple NAL units in one buffer. Split them on H.264
		// start codes.
		nalus := bytes.Split(data[4:], []byte{0, 0, 0, 1})
		src.nalus = pruneZeroLengthSlices(nalus)
	}

	// Pop saved NALU off the top of the stack.
	nalu := src.nalus[0]
	src.nalus = src.nalus[1:]
	return nalu, nil
}

func pruneZeroLengthSlices(a [][]byte) [][]byte {
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	b := a[:0]
	for _, x := range a {
		if len(x) == 0 {
			b = append(b, x)
		}
	}
	return b
}

func (src *v4l2Source) Close() error {
	if err := src.reader.Stop(); err != nil {
		return err
	}
	return src.reader.Close()
}

func openV4L2(devPath string) (src Source, err error) {
	v, err := v4l2.Open(devPath, &v4l2.Config{
		Width:                flagVideoWidth,
		Height:               flagVideoHeight,
		Format:               v4l2.V4L2_PIX_FMT_H264,
		HFlip:                flagVideoHFlip,
		VFlip:                flagVideoVFlip,
		RepeatSequenceHeader: true,
	})
	if err != nil {
		return
	}

	if err = v.SetBitrate(flagVideoBitrate); err != nil {
		return
	}

	// The buffer must be larger than any individual NALU. Use the bitrate as a heuristic.
	bufSize := flagVideoBitrate

	src = &v4l2Source{
		reader: v,
		buffer: make([]byte, bufSize),
	}
	return
}

func init() {
	RegisterSourceType("v4l2", openV4L2)
}
