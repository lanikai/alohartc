package media

// Generic interface for a media (audio or video) source.
type Source interface {
	// RTP payload type information
	PayloadType() string

	// Free up any resources associated with the source
	Close() error
}
