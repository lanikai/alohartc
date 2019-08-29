//////////////////////////////////////////////////////////////////////////////
//
// Stubs for unsupported sinks for compilation and tests to succeed.
//
// Copyright 2019 Lanikai Labs LLC. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

// +build !linux

package media

type ALSAAudioSink struct {
}

func NewALSAAudioSink(devname string) (*ALSAAudioSink, error) {
	return nil, errNotSupported
}

func (as *ALSAAudioSink) Close() error {
	return errNotSupported
}

func (as *ALSAAudioSink) Configure(rate, channels, format int) error {
	return errNotSupported
}

func (as *ALSAAudioSink) Write(p []byte) (int, error) {
	return 0, errNotSupported
}
