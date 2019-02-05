package v4l2

type Config struct {
	Format uint // Video format (e.g. H264)
	Width  uint // Video width in pixels
	Height uint // Video height in pixels

	HFlip bool // Flip video horizontally
	VFlip bool // Flip video vertically

	// Repeat sequence headers (i.e. sequence/picture parameter sets) for
	// H.264 pixel format. This is useful for resynchronization in cases
	// where the parameter sets are lost.
	RepeatSequenceHeader bool
}
