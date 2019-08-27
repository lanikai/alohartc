// +build production,!mp4

package media

func OpenMP4(filename string) (VideoSource, error) {
	panic("MP4 support disabled")
}
