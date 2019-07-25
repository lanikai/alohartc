// +build !production

package media

import (
	"errors"
	"io"
	"time"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/mp4"
)

// Open an MP4 file and return the video stream as a VideoSource.
func OpenMP4(filename string) (VideoSource, error) {
	log.Info("Opening file %s", filename)
	file, err := avutil.Open(filename)
	if err != nil {
		return nil, err
	}

	streams, err := file.Streams()
	if err != nil {
		return nil, err
	}

	var info av.VideoCodecData
	for _, stream := range streams {
		if stream.Type() == av.H264 {
			info = stream.(av.VideoCodecData)
			log.Info("%v stream: %dx%d", info.Type(), info.Width(), info.Height())
		}
	}
	if info == nil {
		return nil, errors.New("No compatible video stream found")
	}

	return &mp4VideoSource{
		filename: filename,
		file:     file,
		info:     info,
	}, nil
}

type mp4File struct {
	filename string
	file     av.DemuxCloser
}

// TODO: Use the same MP4 file as both a VideoSource and an AudioSource.
type mp4VideoSource struct {
	baseSource

	filename string
	file     av.DemuxCloser
	info     av.VideoCodecData
}

func (ms *mp4VideoSource) NewConsumer() *BufferConsumer {
	consumer, n := ms.newConsumer()
	if n == 1 {
		go ms.readLoop()
	}
	return consumer
}

func (ms *mp4VideoSource) Close() error {
	return ms.file.Close()
}

func (ms *mp4VideoSource) Format() string {
	return ms.info.Type().String()
}

func (ms *mp4VideoSource) Width() int {
	return ms.info.Width()
}

func (ms *mp4VideoSource) Height() int {
	return ms.info.Height()
}

// readLoop reads repeatedly from the source file and forwards frames to each
// track. It exits when the file is closed or the last track is removed.
func (ms *mp4VideoSource) readLoop() {
	// When playback started.
	start := time.Now()

	// Time in between each frame, e.g. 40ms for 25 FPS video.
	var interval time.Duration

	// Total number of frames delivered so far.
	count := 0

	write := func(data []byte) {
		buf := NewSharedBuffer(data, nil)
		ms.putBuffer(buf)
	}

	for {
		if ms.numConsumers() == 0 {
			// No consumers left, so no point in continuing.
			return
		}

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

		// TODO: Handle audio stream properly.
		if pkt.Idx != 0 {
			continue
		}

		if interval == 0 {
			// Set frame interval based on time between first and second frame.
			interval = pkt.Time
		}

		// Sleep until ready for next frame: start + count*interval
		delta := time.Until(start.Add(time.Duration(count) * interval))
		time.Sleep(delta)

		var data = pkt.Data[4:]
		if pkt.IsKeyFrame {
			switch info := ms.info.(type) {
			case h264parser.CodecData:
				// Send SPS and PPS along with key frame.
				write(info.SPS())
				write(info.PPS())
				data = skipSEI(data)
			default:
				log.Warn("Unrecognized video codec: %T", ms.info)
			}
		}

		write(data)

		count++

		log.Debug("Packet: %7d bytes, starting with %02x", len(data), data[0:4])
	}
}

// reset seeks to the beginning of the input file.
func (ms *mp4VideoSource) reset() {
	log.Debug("Seeking to beginning of MP4 file")
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

func init() {
	avutil.DefaultHandlers.Add(mp4.Handler)
}
