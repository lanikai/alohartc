package media

import (
	"flag"
)

type VideoSource interface {
	Source
}

// An H.264 video source.
type H264Source interface {
	VideoSource

	// Read one whole NAL unit. On EOF, return an empty byte slice.
	//
	// The returned slice is valid only until the next call to ReadNALU().
	ReadNALU() ([]byte, error)
}

var (
	flagVideoBitrate int
	flagVideoWidth   uint
	flagVideoHeight  uint
	flagVideoHflip   bool
	flagVideoVflip   bool
)

func init() {
	flag.IntVar(&flagVideoBitrate, "b", 2e6, "Video bitrate")
	flag.UintVar(&flagVideoWidth, "w", 1280, "Video width")
	flag.UintVar(&flagVideoHeight, "h", 720, "Video height")
	flag.BoolVar(&flagVideoHflip, "hflip", false, "Flip video horizontally")
	flag.BoolVar(&flagVideoVflip, "vflip", false, "Flip video vertically")
}
