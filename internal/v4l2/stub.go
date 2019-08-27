// +build !linux release,!v4l2

package v4l2

import (
	"github.com/lanikai/alohartc/internal/media"
)

func Open(devpath string, cfg Config) (media.VideoSource, error) {
	panic("V4L2 support disabled")
}
