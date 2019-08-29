//////////////////////////////////////////////////////////////////////////////
//
// Media sinks unique to Linux:
//
// * ALSAAudioSink: Advanced Linux Sound Architecture (ALSA) audio sink
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

// +build linux

package media

// #cgo pkg-config: alsa
// #include <stdlib.h>
// #include <alsa/asoundlib.h>
import "C"
import (
	"errors"
	"unsafe"
)

// ALSAAudioSink writes audio to an ALSA soundcard for playback
type ALSAAudioSink struct {
	handle    *C.struct__snd_pcm
	framesize int
}

// NewALSAAudioSink returns an ALSA audio consumer
func NewALSAAudioSink(devname string) (*ALSAAudioSink, error) {
	as := &ALSAAudioSink{}

	// Open ALSA playback device
	name := C.CString(devname)
	err := C.snd_pcm_open(&as.handle, name, C.SND_PCM_STREAM_PLAYBACK, 0)
	C.free(unsafe.Pointer(name))
	if err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	return as, nil
}

// Close ALSA playback device
func (as *ALSAAudioSink) Close() error {
	// Drop remaining unprocessed samples in the buffer
	if err := C.snd_pcm_drop(as.handle); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Close playback device
	if err := C.snd_pcm_close(as.handle); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return nil
}

// Configure ALSA playback device
func (as *ALSAAudioSink) Configure(rate, channels, format int) error {
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

	// Set sample format
	switch format {
	case S8:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_S8,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
		as.framesize = 1 * channels
	case U8:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_U8,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
		as.framesize = 1 * channels
	case S16LE:
		if err := C.snd_pcm_hw_params_set_format(
			as.handle,
			hwparams,
			C.SND_PCM_FORMAT_S16_LE,
		); err < 0 {
			return errors.New(C.GoString(C.snd_strerror(err)))
		}
		as.framesize = 2 * channels
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

	// Set playback device parameters
	if err := C.snd_pcm_hw_params(as.handle, hwparams); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Free parameters struct
	C.snd_pcm_hw_params_free(hwparams)

	// Set playback device parameters
	if err := C.snd_pcm_prepare(as.handle); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}

	return nil
}

// Write audio byte buffer to the playback device
func (as *ALSAAudioSink) Write(p []byte) (int, error) {
	numframes := len(p) / as.framesize
	buf := C.CBytes(p)
	n := C.snd_pcm_writei(as.handle, buf, C.ulong(numframes))
	C.free(buf)
	if n < 0 {
		if err := C.snd_pcm_recover(as.handle, C.int(n), 0); err < 0 {
			return 0, errors.New(C.GoString(C.snd_strerror(err)))
		}
		return 0, nil
	}

	return int(n), nil
}
