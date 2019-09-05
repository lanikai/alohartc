package media

// AudioSink is the interface that extends the basic MediaSink interface for
// audio sinks, typically playback devices
type AudioSink interface {
	MediaSink

	// Configure audio sink sample rate, number of channels, and sample format
	Configure(rate int, channels int, format int) error
}
