package media

// AudioSource is the interface that extends the basic MediaSource
// interface for audio sources
type AudioSource interface {
	MediaSource

	Configure(rate, channels, format int) error
}
