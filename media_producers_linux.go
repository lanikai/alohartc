//////////////////////////////////////////////////////////////////////////////
//
// Media producers unique to Linux
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

type ALSAAudioProducer struct {
	pcm      *C.struct__snd_pcm
	hwparams *C.struct__snd_pcm_hw_params
}

// NewALSAAudioProducer opens a specific capture device
// Use "default" for devname to open the default capture device.
// Use "hw:1" for devname to open the seeed-2mic-voicecard on a RPi as
// "hw:0" is the on-board soundcard, which does not support capture.
func NewALSAAudioProducer(devname string) (*ALSAAudioProducer, error) {
	p := &ALSAAudioProducer{}

	// Open ALSA capture device
	name := C.CString(devname)
	err := C.snd_pcm_open(&p.pcm, name, C.SND_PCM_STREAM_CAPTURE, 0)
	C.free(unsafe.Pointer(name))
	if err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Allocate hardware parameters structure
	if err := C.snd_pcm_hw_params_malloc(&p.hwparams); err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Initialize hardware parameters structure
	if err := C.snd_pcm_hw_params_any(p.pcm, p.hwparams); err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	// Set access type
	if err := C.snd_pcm_hw_params_set_access(
		p.pcm,
		p.hwparams,
		C.SND_PCM_ACCESS_RW_INTERLEAVED,
	); err < 0 {
		return nil, errors.New(C.GoString(C.snd_strerror(err)))
	}

	return p, nil
}

// Close ALSA capture device
func (p *ALSAAudioProducer) Close() error {
	// Free hardware parameters struct
	C.snd_pcm_hw_params_free(p.hwparams)

	// Close capture device
	if err := C.snd_pcm_close(p.pcm); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return nil
}

// SetSampleRate of capture device
func (p *ALSAAudioProducer) SetSampleRate(sr int) error {
	if err := C.snd_pcm_hw_params_set_rate(
		p.pcm,
		p.hwparams,
		C.uint(sr),
		0,
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return nil
}

// SetSampleSize of capture device
func (p *ALSAAudioProducer) SetSampleSize(format int) error {
	if err := C.snd_pcm_hw_params_set_format(
		p.pcm,
		p.hwparams,
		C.SND_PCM_FORMAT_S16_LE,
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return nil
}

// SetNumChannels of capture device
func (p *ALSAAudioProducer) SetNumChannels(nc int) error {
	if err := C.snd_pcm_hw_params_set_channels(
		p.pcm,
		p.hwparams,
		C.uint(nc),
	); err < 0 {
		return errors.New(C.GoString(C.snd_strerror(err)))
	}
	return nil
}
