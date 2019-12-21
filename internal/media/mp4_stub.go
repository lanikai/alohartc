// +build production,!mp4

package media

import "errors"

func OpenMP4(filename string) (VideoSource, error) {
	return nil, errors.New("MP4 support disabled")
}
