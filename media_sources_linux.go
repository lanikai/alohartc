//////////////////////////////////////////////////////////////////////////////
//
// Media sources unique to Linux:
//
// * ALSAAudioSource: Advanced Linux Sound Architecture (ALSA) audio source
// * V4L2VideoSource: Video4Linux2 video source
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

// +build linux

package alohartc

// #cgo pkg-config: alsa
// #include <stdlib.h>
// #include <alsa/asoundlib.h>
import "C"
import (
	"errors"
	"unsafe"
)

// ALSAAudioSource captures audio from an ALSA soundcard
type ALSAAudioSource struct {
	bcast          *Broadcaster
	handle         *C.struct__snd_pcm
	stop           chan bool
	numSubscribers int
}

// NewALSAAudioSource opens a specific capture device
// Use "default" for devname to open the default capture device.
// Use "hw:1" for devname to open the seeed-2mic-voicecard on a RPi as
// "hw:0" is the on-board soundcard, which does not support capture.
func NewALSAAudioSource(devname string) (*ALSAAudioSource, error) {
	as := &ALSAAudioSource{
		bcast: NewBroadcaster(),
		stop:  make(chan bool),
	}

	// Open ALSA capture device
	name := C.CString(devname)
	err := C.snd_pcm_open(&as.handle, name, C.SND_PCM_STREAM_CAPTURE, 0)
	C.free(unsafe.Pointer(name))
	if err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	return as, nil
}

// Configure ALSA capture device
func (as *ALSAAudioSource) Configure(rate, channels, format int) error {
	var hwparams *C.struct__snd_pcm_hw_params

	// Allocate hardware parameters structure
	if err := C.snd_pcm_hw_params_malloc(&hwparams); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Initialize hardware parameters structure
	if err := C.snd_pcm_hw_params_any(as.handle, hwparams); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Set access type
	if err := C.snd_pcm_hw_params_set_access(
		as.handle,
		hwparams,
		C.SND_PCM_ACCESS_RW_INTERLEAVED,
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Set format
	switch format {
	case S8:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_S8,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
	case U8:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_U8,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
	case S16LE:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_S16_LE,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
	default:
		return errNotImplemented
	}

	// Set number of channels
	if err := C.snd_pcm_hw_params_set_channels(
		as.handle,
		hwparams,
		C.uint(channels),
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Set sample rate
	if err := C.snd_pcm_hw_params_set_rate(
		as.handle,
		hwparams,
		C.uint(rate),
		0,
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Free hardware parameters struct
	C.snd_pcm_hw_params_free(hwparams)

	// Prepare for use
	if err := C.snd_pcm_prepare(as.handle); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	return nil
}

// Close ALSA capture device
func (as *ALSAAudioSource) Close() error {
	// Close capture device
	if err := C.snd_pcm_close(as.handle); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return as.bcast.Close()
}

// Subscribe to frames from capture device. Returned buffered channel will
// receive frames from capture device, buffering up to n frames. Underlying
// frame data is shared amongst all other subscribers -- do not modify.
func (as *ALSAAudioSource) Subscribe(n int) <-chan []byte {
	// Start capturing if first subscriber
	if 0 == as.numSubscribers {
		go as.capture()
	}
	as.numSubscribers++

	return as.bcast.Subscribe(n)
}

// Unsubscribe from capture device. Channel will receive no more frames.
func (as *ALSAAudioSource) Unsubscribe(ch <-chan []byte) error {
	as.numSubscribers--
	if 0 == as.numSubscribers {
		as.stop <- true
	}
	return as.bcast.Unsubscribe(ch)
}

// capture buffers from device and write to broadcaster
func (as *ALSAAudioSource) capture() {
	for {
		select {
		case <-as.stop:
			return
		default:
			// Capture
			out := C.malloc(bytesPerSample * numChannels * numFrames)
			n := C.snd_pcm_readi(
				as.handle,
				unsafe.Pointer(out),
				C.snd_pcm_uframes_t(numFrames),
			)
			if n < 0 {
				log.Println(C.GoString(C.snd_strerror(C.int(n))))
			}
			raw := C.GoBytes(out, bytesPerSample*numChannels*n)
			C.free(unsafe.Pointer(out))

			// Encode
			encoded := raw

			// Broadcast
			as.bcast.Write(encoded)
		}
	}
}

// TODO Port v4l2 module to a cgo-based source using videodev2.h. More
//      robust, and now need cgo anyhow for libasound2 and libopus.
type V4L2VideoSource struct {
	VideoSourcer
}
