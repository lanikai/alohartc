// +build !production

package media

import (
	"container/list"
	"io"
	"os"
	"sync"
	"time"

	errors "golang.org/x/xerrors"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/codec/h264parser"
	"github.com/nareix/joy4/format/mp4"
)

// Open an MP4 file and return the video stream as a VideoSource.
func OpenMP4(filename string) (VideoSource, error) {
	log.Info("Opening file %s", filename)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	demuxer := mp4.NewDemuxer(file)

	streams, err := demuxer.Streams()
	if err != nil {
		return nil, err
	}

	var info av.VideoCodecData
	for _, stream := range streams {
		if stream.Type() == av.H264 {
			info = stream.(av.VideoCodecData)
			log.Info("%v stream: %dx%d", info.Type(), info.Width(), info.Height())
		} else {
			log.Debug("Skipping %v stream", stream.Type())
		}
	}
	if info == nil {
		return nil, errors.New("No compatible video stream found")
	}

	f := &mp4File{
		file:      file,
		demuxer:   demuxer,
		readahead: newPacketQueue(),
	}

	return &mp4VideoSource{
		f:    f,
		idx:  0,
		info: info,
	}, nil
}

const maxReadahead = 16

type packetQueue struct {
	*list.List
	maxLen int
}

func newPacketQueue() packetQueue {
	return packetQueue{list.New()}
}

func (q *packetQueue) push(pkt *av.Packet) {
	if q.Len() < maxReadahead {
		// Add new pkt to the end of the queue.
		q.PushBack(pkt)
	} else {
		// Prevent unbounded queue growth by replacing the oldest element (the
		// front) with the new pkt.
		e := q.Front()
		e.Value = pkt
		q.MoveToBack(e)
	}
}

// Pop the first packet with the given stream index, or nil if none found.
func (q *packetQueue) pop(streamIdx int8) *av.Packet {
	for e := q.Front(); e != nil; e = e.Next() {
		pkt := e.Value.(*av.Packet)
		if pkt.Idx == streamIdx {
			q.Remove(e)
			return pkt
		}
	}
	return nil
}

type mp4File struct {
	file    *os.File
	demuxer *mp4.Demuxer

	// Bitmask of streams we care about.
	//streamMask uint32

	// Readahead queue of audio/video packets.
	readahead packetQueue

	sync.Mutex
}

func (f *mp4File) readNext(streamIdx int8) *av.Packet {
	f.Lock()
	defer f.Unlock()

	// Check readahead queue in case we already have a packet ready.
	pkt := f.readahead.pop(streamIdx)
	if pkt != nil {
		return pkt
	}

	// Otherwise, read from the demuxer until we find a packet.
	for {
		pkt, err := f.demuxer.ReadPacket()
		if err == io.EOF {
			f.demuxer.SeekToTime(0)
			continue
		} else if err != nil {
			log.Error("Error reading packet from %s: %v", f.file.Name(), err)
			return nil
		}

		if pkt.Idx == streamIdx {
			return &pkt
		} else {
			// Leave other packets on the readahead queue.
			f.readahead.push(&pkt)
		}
	}
}

type mp4AudioSource struct {
	// TODO
}

type mp4VideoSource struct {
	f *mp4File

	// Stream index of this video stream.
	idx int8

	baseSource
	info     av.VideoCodecData
	interval time.Duration
}

func (vs *mp4VideoSource) StartReceiving() <-chan *SharedBuffer {
	bufCh, n := vs.startRecv(0)
	if n == 1 {
		go vs.readLoop()
	}
	return bufCh
}

func (vs *mp4VideoSource) StopReceiving(bufCh <-chan *SharedBuffer) {
	vs.stopRecv(bufCh)
}

func (vs *mp4VideoSource) Close() error {
	// TODO
	return nil
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

func (vs *mp4VideoSource) readLoop() {
	// Last packet to be processed.
	var lastPkt *av.Packet

	// Wall clock time when the last packet was due to be sent.
	var lastTime time.Time

	write := func(data []byte) {
		buf := NewSharedBuffer(data, nil)
		vs.putBuffer(buf)
	}

	for {
		if vs.numReceivers() == 0 {
			// No point in continuing if there are no receivers.
			return
		}

		pkt := vs.f.readNext(vs.idx)
		if pkt == nil {
			return
		}

		if lastPkt != nil && pkt.Time > lastPkt.Time {
			// Sleep until it's time for this packet. (The first packet is
			// processed right away.)
			frameTime := lastTime.Add(pkt.Time - lastPkt.Time)
			time.Sleep(time.Until(frameTime))
			lastTime = frameTime
		} else {
			// First packet, or MP4 file looped back to the beginning.
			lastTime = time.Now()
		}
		lastPkt = pkt

		var data = pkt.Data[4:]
		if pkt.IsKeyFrame {
			switch info := vs.info.(type) {
			case h264parser.CodecData:
				// Send SPS and PPS along with key frame.
				write(info.SPS())
				write(info.PPS())
				data = skipSEI(data)
			default:
				log.Warn("Unrecognized video codec: %T", vs.info)
			}
		}

		write(data)

		log.Debug("Packet: %6d bytes, starting with %02x", len(data), data[0:4])
	}
}

// Read the first few packets of the MP4 file to identify the frame interval.
func determineFrameInterval(demuxer *mp4.Demuxer) (time.Duration, error) {
	interval := time.Duration(0)
	for interval == 0 {
		pkt, err := demuxer.ReadPacket()
		if err != nil {
			return 0, err
		}
		if pkt.Idx == 0 {
			interval = pkt.Time
		}
	}

	if err := demuxer.SeekToTime(0); err != nil {
		return 0, errors.Errorf("Seek failed: %v", err)
	}

	return interval, nil
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
