// +build !production

package media

import (
	"errors"
	"io"
	"os"
	"time"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/mp4"
)

// Open an MP4 file and return the video stream as a VideoSource.
// TODO: Return an AudioSource as well.
func OpenMP4(filename string) (VideoSource, error) {
	log.Info("Opening file %s", filename)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	demuxer := mp4.NewDemuxer(file)

	codecs, err := demuxer.Streams()
	if err != nil {
		return nil, err
	}

	f := &mp4File{
		file:    file,
		demuxer: demuxer,
		codecs:  codecs,
	}

	var video *mp4VideoSource
	for _, codec := range codecs {
		switch codec.Type() {
		case av.H264:
			info := codec.(av.VideoCodecData)
			log.Info("%v stream: %dx%d", info.Type(), info.Width(), info.Height())
			video = &mp4VideoSource{f: f, info: info}
			f.sources = append(f.sources, &video.baseSource)
		default:
			log.Debug("Skipping %v stream", codec.Type())
			f.sources = append(f.sources, nil)
		}
	}

	if video == nil {
		return nil, errors.New("No compatible video stream found")
	}

	loop := &singletonLoop{
		run: f.readLoop,
	}
	video.start = loop.start
	video.stop = loop.stop

	return video, nil
}

type mp4File struct {
	file    *os.File
	demuxer *mp4.Demuxer

	codecs  []av.CodecData
	sources []*baseSource
}

func (f *mp4File) readLoop(quit <-chan struct{}) {
	// Wall clock offset to the first packet in the file.
	var start time.Time

	for {
		select {
		case <-quit:
			return
		default:
		}

		// Read the next packet from the file.
		pkt, err := f.demuxer.ReadPacket()

		if err == io.EOF {
			// Add a 50 millisecond delay, then play the file again.
			f.demuxer.SeekToTime(0)
			start = time.Now().Add(50 * time.Millisecond)
			continue
		} else if err != nil {
			log.Error("Error reading packet from %s: %v", f.file.Name(), err)
			return
		}

		codec := f.codecs[pkt.Idx]
		src := f.sources[pkt.Idx]
		if src == nil {
			continue
		}

		if start.IsZero() {
			// The read loop might start in the middle of the file, so
			// initialize the start offset accordingly. This first packet will
			// be presented immediately.
			start = time.Now().Add(-pkt.Time)
		} else {
			// Sleep until this packet is ready to be presented.
			time.Sleep(time.Until(start.Add(pkt.Time)))
		}

		data := pkt.Data[4:]

		if pkt.IsKeyFrame {
			// Codec-specific processing.
			switch cd := codec.(type) {
			case h264parser.CodecData:
				// Send SPS and PPS along with key frame.
				src.putBuffer(cd.SPS(), nil)
				src.putBuffer(cd.PPS(), nil)
				data = skipSEI(data)
			}
		}

		src.putBuffer(data, nil)

		log.Debug("Packet: %6d bytes, starting with %02x", len(data), data[0:4])
	}
}

type mp4AudioSource struct {
	// TODO
}

type mp4VideoSource struct {
	baseSource

	f *mp4File

	info av.VideoCodecData
}

func (vs *mp4VideoSource) Codec() string {
	return vs.info.Type().String()
}

func (vs *mp4VideoSource) Width() int {
	return vs.info.Width()
}

func (vs *mp4VideoSource) Height() int {
	return vs.info.Height()
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
