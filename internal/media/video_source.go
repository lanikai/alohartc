package media

type VideoSource interface {
	MediaSource

	Codec() string

	Width() int
	Height() int
}
