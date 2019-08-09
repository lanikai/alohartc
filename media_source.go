//////////////////////////////////////////////////////////////////////////////
//
// MediaSource defines an interface for audio and video sources
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"io"
	"os"
)

// Second attempt: Emulate the WebRTC API

// NOTE Emulating the WebRTC API may not be feasible. MediaStreamTrack
//      does not address media internals, such as codecs, forcing IDR for
//      H.264 when FIR received from remote, etc. These are likely handled
//      iternally via the MediaStreamTrack UUID.

// pc := NewRTCPeerConnection()
// md := NewKunaMediaDevices()
// //md := NewRpiMediaDevices()
// ms, _ := md.GetUserMedia(&MediaStreamConstraints{
//   audio: true,
//   video: true,
// })
// for _, track := range(ms.GetTracks()) {
//   pc.AddTrack(track)
// }

type MediaStreamConstraints struct {
	Audio bool `json:"audio"`
	Video bool `json:"video"`
}

type MediaDevices interface {
	GetUserMedia(msc *MediaStreamConstraints) (ms *MediaStreamer, err error)
}

type KunaMediaDevices struct {
}

func (md *KunaMediaDevices) GetUserMedia(msc *MediaStreamConstraints) (ms *MediaStreamer, err error) {
	return nil, errNotImplemented
}

type RPiMediaDevices struct {
}

func NewRPiMediaDevices() *RPiMediaDevices {
	return nil
}

func (md *RPiMediaDevices) GetUserMedia(msc *MediaStreamConstraints) (ms *MediaStreamer, err error) {
	return nil, errNotImplemented
}

type FileMediaDevices struct {
}

func NewFileMediaDevices() *FileMediaDevices {
	return nil
}

func (md *FileMediaDevices) GetUserMedia(msc *MediaStreamConstraints) (ms *MediaStreamer, err error) {
	return nil, errNotImplemented
}

type MediaTrackConstraints struct {
	AutoGainControl bool
}

type MediaStreamTrack struct {
	Kind string `json:"kind"` // "audio" or "video"
}

func (mst *MediaStreamTrack) GetConstraints() *MediaTrackConstraints {
	return nil
}

type MediaStreamer interface {
	AddTrack(mst *MediaStreamTrack)
	GetAudioTracks() []*MediaStreamTrack
	GetTracks() []*MediaStreamTrack
	GetVideoTracks() []*MediaStreamTrack
}

type MediaStream struct {
	MediaStreamer

	Active bool     `json:"active"`
	Ended  bool     `json:"ended"`
	Id     [36]byte `json:"id"`
}

func (ms *MediaStream) AddTrack(t *MediaStreamTrack) {
}

func (ms *MediaStream) GetAudioTracks() []*MediaStreamTrack {
	return nil
}

func (ms *MediaStream) GetTracks() []*MediaStreamTrack {
	return nil
}

func (ms *MediaStream) GetVideoTracks() []*MediaStreamTrack {
	return nil
}

// AddTrack adds a new media track to the set of tracks which will be
// transmitted to the other peer.
func (pc *PeerConnection) AddTrack(t *MediaStreamTrack) error {
	return nil
}

// First attempt: Producer and Consumer interfaces

// * If possible, be able to export C interface which emulates WebRTC API
//   Can interfaces be used from C?
// * Some producers may have both audio and video (e.g. RTSP)
// * Some producers output encoded data (e.g. V4L2). Other producers output
//   data which must be encoded (e.g. ALSA soundard), V4L2 YUYV.
// * WebRTC should be able to actuate producer (e.g. adjust bitrate, adjust
//   framerate, force and IDR)

// MediaProducer is the interface used for providers of media, such as
// microphones and cameras.
//
// Multiple readers must be supported and each reader must receive a copy of
// source content from the time they join. How much content is buffered if
// a reader fails to read in time is left to the implementation.
type MediaProducer interface {
	io.Closer

	// Codec returns the codec used by the producer
	Codec() string
}

// AudioProducer is the interface that extends the basic MediaProducer
// interface for audio producers.
//
// TODO Can we get away with encoding once? For instance, say we're using
//      Opus, which supports error correction. Say there are multiple
//      viewers. Can the same encoded byte stream support all viewers?
//      Or does each viewer (i.e. each Track) need own encoder?
type AudioProducer interface {
	MediaProducer

	// AudioTrack get a new audio track from the producer. Closing the
	// track should tell the consumer no more reads will occur.
	//
	// Multiple simultaneous subscribers must be supported.
	AudioTrack() (*io.ReadCloser, error)

	// SetSamplerate for the audio source. Not to be called mid-stream.
	SetSampleRate(sr int) error

	// SetNumChannels for the audio source. Not to be called mid-stream.
	SetNumChannels(nc int) error

	// SetSampleSize for the audio source. Not to be called mid-stream.
	SetSampleSize(ss int) error
}

type AudioTrackProducer interface {
}

// VideoProducer is the interface that extends the basic MediaProducer
// interface for video producers.
type VideoProducer interface {
	MediaProducer

	// VideoTrack get a new audio track from the producer. Closing the
	// track should tell the consumer no more reads will occur.
	VideoTrack() (*VideoTrack, error)
}

type VideoTrack interface {
	io.ReadCloser

	// Force video source to produce an IDR rate.
	// If not supported, return errNotSupported.
	ForceIDR() error

	// SetBitrate for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetBitRate(br int) error

	// SetFramerate for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetFrameRate(fr int) error

	// SetFramesize for the video source. May be called mid-stream.
	// If not supported, return errNotSupported.
	SetFrameSize(fs int) error
}

type ALSAAudioProducer struct {
	AudioProducer
}

func NewALSAAudioProducer() (*ALSAAudioProducer, error) {
	return nil, errNotImplemented
}

type V4L2VideoProducer struct {
	VideoProducer
}

// NOTE Since we need a cross-compiler now for anyhow for libaudio2.h and
//      libopus.h, we might as well use cgo for V4L2 as well -- the
//      cross-compiler toolchains include linux/videodev2.h and it would
//      be more robust than the current "hardcoded magic values" used
//      currently in unix.Syscall.
func NewV4L2VideoProducer(devname string) (*V4L2VideoProducer, error) {
	return nil, errNotImplemented
}

type RTSPAudioVideoProducer struct {
	AudioProducer
	VideoProducer
}

func NewRTSPAudioVideoProducer(url string) (*RTSPAudioVideoProducer, error) {
	return nil, errNotImplemented
}

// MediaConsumer is the interface for devices which consume media, such as
// soundcard outputs and video displays.
type MediaConsumer interface {
	io.Closer
	io.Writer

	// SetCodec to be expected by consumer. Consumers may support multiple
	// codecs.
	SetCodec(c string) error
}

type FileMediaProducer struct {
	file   *os.File
	tracks []*Track

	active bool
}

func NewFileMediaProducer(filename string) (*FileMediaProducer, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return &FileMediaProducer{file: f}, nil
}

// ALSAAudioConsumer writes audio to an ALSA soundcard
//
// Audio must be decoded before writing to soundcard (e.g. Opus)
type ALSAAudioConsumer struct {
	numChannels int
	sampleRate  int
	sampleSize  int
}

// NewALSAAudioConsumer returns an ALSA audio consumer
func NewALSAAudioConsumer(numChannels, sampleRate, sampleSize int) *ALSAAudioConsumer {
	return nil
}

// MediaSource is the interface
type MediaSource interface {
	Close() error
	GetTrack() Track
	CloseTrack(Track)
}
