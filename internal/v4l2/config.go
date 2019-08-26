package v4l2

type Config struct {
	Format int // Video format (e.g. H264)
	Width  int // Video width in pixels
	Height int // Video height in pixels

	Bitrate int

	// Repeat sequence headers (i.e. sequence/picture parameter sets) for
	// H.264 pixel format. This is useful for resynchronization in cases
	// where the parameter sets are lost.
	RepeatSequenceHeader bool
}
