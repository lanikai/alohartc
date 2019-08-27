// +build !release

//////////////////////////////////////////////////////////////////////////////
//
// FileMediaSource implements a read-from-file media source
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

package alohartc

import (
	"errors"
	"io"
	"time"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/mp4"
)

func init() {
	avutil.DefaultHandlers.Add(mp4.Handler)
}

type pipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

type FileMediaSource struct {
	filename string
	file     av.DemuxCloser
	tracks   map[Track]pipe
	video    av.VideoCodecData
}

// NewFileMediaSource creates a file source
func NewFileMediaSource(filename string) (*FileMediaSource, error) {
	log.Info("Opening file %s", filename)
	file, err := avutil.Open(filename)
	if err != nil {
		return nil, err
	}

	streams, err := file.Streams()
	if err != nil {
		return nil, err
	}

	var video av.VideoCodecData
	for _, stream := range streams {
		if stream.Type() == av.H264 {
			video = stream.(av.VideoCodecData)
			log.Info("%v stream: %dx%d", video.Type(), video.Width(), video.Height())
		}
	}
	if video == nil {
		return nil, errors.New("No compatible video stream found")
	}

	ms := &FileMediaSource{
		filename: filename,
		file:     file,
		tracks:   make(map[Track]pipe),
		video:    video,
	}
	return ms, nil
}

// Close file media source
func (ms *FileMediaSource) Close() error {
	return ms.file.Close()
}

type h264Track struct {
	io.ReadCloser
}

func (t *h264Track) PayloadType() string {
	return "H264/90000"
}

// GetTrack opens file source as new reader.
func (ms *FileMediaSource) GetTrack() Track {
	// Create a new pipe
	reader, writer := io.Pipe()
	p := pipe{reader, writer}

	// Create a new track
	track := &h264Track{reader}
	ms.tracks[track] = p

	if len(ms.tracks) == 1 {
		// First track, start the read loop.
		ms.reset()
		go ms.readLoop()
	}

	return track
}

// CloseTrack closes file source reader.
func (ms *FileMediaSource) CloseTrack(track Track) {
	if p, ok := ms.tracks[track]; ok {
		// Delete the track and close the pipe writer.
		// Reader will return io.EOF.
		delete(ms.tracks, track)
		p.writer.Close()
	}
}

// Seek to beginning of input file.
func (ms *FileMediaSource) reset() {
	log.Debug("Seeking to beginning of file")
	h := ms.file.(*avutil.HandlerDemuxer)
	if d, ok := h.Demuxer.(*mp4.Demuxer); ok {
		err := d.SeekToTime(0)
		if err != nil {
			log.Error("Seek failed: %v", err)
		}
	} else {
		log.Error("Expected a mp4.Demuxer, not %T", h.Demuxer)
	}
}

// readLoop reads repeatedly from the source file and forwards frames to each
// track. It exits when the file is closed or the last track is removed.
func (ms *FileMediaSource) readLoop() {
	// When playback started.
	start := time.Now()

	// Time in between each frame, e.g. 40ms for 25 FPS video.
	var interval time.Duration

	// Total number of frames delivered so far.
	count := 0

	for {
		pkt, err := ms.file.ReadPacket()
		if err != nil {
			if err == io.EOF {
				ms.reset()
				continue
			} else {
				log.Error("Error reading packet from %s: %v", ms.filename, err)
				return
			}
		}

		if interval == 0 {
			// Set frame interval based on time between first and second frame.
			interval = pkt.Time
		}

		// Sleep until ready for next frame: start + count*interval
		delta := time.Until(start.Add(time.Duration(count) * interval))
		time.Sleep(delta)

		var writers []io.Writer
		for _, p := range ms.tracks {
			writers = append(writers, p.writer)
		}
		if writers == nil {
			// No tracks, stop reading.
			return
		}
		mw := io.MultiWriter(writers...)

		var data = pkt.Data[4:]
		if pkt.IsKeyFrame {
			switch v := ms.video.(type) {
			case h264parser.CodecData:
				// Send SPS and PPS along with key frame.
				mw.Write(v.SPS())
				mw.Write(v.PPS())
				data = skipSEI(data)
			default:
				log.Warn("Unrecognized video type: %T", ms.video)
			}
		}

		mw.Write(data)
		count++

		log.Debug("Packet: %7d bytes, starting with %02x", len(data), data[0:4])
	}
}

// Skip past the SEI (if present) in a H.264 data packet.
// See ITU-T H.264 section 7.3.2.3.
func skipSEI(data []byte) []byte {
	if len(data) == 0 || data[0] != 0x06 {
		// No SEI in this packet.
		return data
	}

	// First parse and discard payload type.
	i := 1
	payloadType := 0
	for data[i] == 0xff {
		payloadType += 255
		i++
	}
	payloadType += int(data[i])
	log.Debug("SEI payload type: %d", payloadType)
	i++

	// Now parse the payload size.
	size := 0
	for data[i] == 0xff {
		size += 255
		i++
	}
	size += int(data[i])
	log.Debug("SEI payload size: %d", size)
	i++

	// TODO: Why +5 ?
	return data[i+size+5:]
}
