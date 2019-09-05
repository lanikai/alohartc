//////////////////////////////////////////////////////////////////////////////
//
// Stub functions for operating systems on which ALSA is not supported.
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

// +build !linux

package media

type ALSAAudioSource struct {
}

func NewALSAAudioSource(devname string) (*ALSAAudioSource, error) {
	return nil, errNotSupported
}

func (as *ALSAAudioSource) Close() error {
	return errNotSupported
}

func (as *ALSAAudioSource) Configure(rate, channels, format int) error {
	return errNotSupported
}

func (as *ALSAAudioSource) Subscribe(n int) <-chan []byte {
	return nil
}

func (as *ALSAAudioSource) Unsubscribe(ch <-chan []byte) error {
	return errNotSupported
}
