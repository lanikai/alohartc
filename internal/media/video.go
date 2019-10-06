package media

type VideoSource interface {
	Source

	Codec() string

	Width() int
	Height() int

	SetBitrate(bps int) error

	//GetFramerate() float32
	//GetBitrate() int

	//AdjustBitrate(bps int)
}
