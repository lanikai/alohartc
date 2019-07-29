package media

type AudioSource interface {
	Source

	Codec() string

	SampleRate() int
	BytesPerSample() int

	//GetFramerate() float32
	//GetBitrate() int

	//AdjustBitrate(bps int)
}
