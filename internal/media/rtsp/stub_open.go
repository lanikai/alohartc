// +build !rtsp

package rtsp

import (
	"github.com/lanikai/alohartc/internal/media"
)

func Open(uri string) (media.VideoSource, error) {
	panic("RTSP support disabled")
}
