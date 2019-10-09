// +build v4l2 !production
// +build linux

package v4l2

import (
	"bytes"

	"github.com/lanikai/alohartc/internal/media"
)

// Open a V4L2 video device (usually /dev/video0).
func Open(devpath string, cfg Config) (media.VideoSource, error) {
	dev, err := OpenDevice(devpath)
	if err != nil {
		return nil, err
	}

	if cfg.Width <= 0 {
		cfg.Width = 1280
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}
	if cfg.Format <= 0 {
		cfg.Format = V4L2_PIX_FMT_H264
	}
	if err := dev.SetPixelFormat(cfg.Width, cfg.Height, cfg.Format); err != nil {
		return nil, err
	}

	if cfg.Bitrate > 0 {
		if err := dev.SetBitrate(cfg.Bitrate); err != nil {
			return nil, err
		}
	}

	if err := dev.SetRepeatSequenceHeader(cfg.RepeatSequenceHeader); err != nil {
		return nil, err
	}

	v := &videoSource{
		cfg: cfg,
		dev: dev,
	}
	v.Flow.Start = func() {
		if err := dev.Start(); err != nil {
			// TODO: Proper error handling.
			panic(err)
		}

		go func() {
			for {
				buf, err := dev.ReadFrame()
				if err != nil {
					v.Flow.Shutdown(err)
					break
				}
				// On the Raspberry Pi, each picture NALU is delivered as a
				// separate buffer, prefixed by an Annex-B start code. But
				// SPS/PPS/SEI may come concatenated together, so to be safe we
				// always split.
				for _, nalu := range bytes.Split(buf, []byte{0, 0, 0, 1}) {
					if len(nalu) > 0 {
						log.Debug("nalu = % 5d bytes, %02x", len(nalu), nalu[0:2])
						v.Flow.PutBuffer(nalu, nil)
					}
				}
			}
		}()
	}
	v.Flow.Stop = func() {
		dev.Stop()
	}
	return v, nil
}

// A media.VideoSource wrapping a V4L2 device.
type videoSource struct {
	media.Flow

	quit chan struct{}

	cfg Config

	dev *device

	current int
}

func (v *videoSource) Codec() string {
	return "H264"
}

func (v *videoSource) Width() int {
	return v.cfg.Width
}

func (v *videoSource) Height() int {
	return v.cfg.Height
}

func (v *videoSource) SetBitrate(target int) error {
	// TODO this control loop should probably be source agnostic. move elsewhere?

	max := 3000000
	min := 30000 // same as chrome

	// Upper bound. No perceptual difference abover 3Mbps.
	if target > max {
		target = max
	}
	if target < min {
		// TODO googSuspectBelowMinBitrate option
		target = min
	}

	delta := target - v.current

	// If available bitrate is less than current, or significant jump upward,
	// adjust immediately
	if target < v.current || (float32(delta) > (0.1 * float32(v.current))) {
		v.current = target
		log.Info("adjusting bitrate: %v", v.current)
		return v.dev.SetBitrate(v.current)
	}

	return nil
}
