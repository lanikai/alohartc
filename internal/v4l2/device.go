// +build v4l2 !release
// +build linux

package v4l2

import (
	"io"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// A V4L2 character device.
type device struct {
	// Number of requested kernel driver buffers.
	// TODO: Currently only numBuffers = 1 is supported.
	numBuffers int

	// Device path, usually "/dev/video0".
	path string

	// File descriptor of v4l2 device.
	fd int

	// Memory-mapped buffer.
	mmap []byte
}

func OpenDevice(path string) (*device, error) {
	fd, err := unix.Open(path, unix.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	return &device{
		numBuffers: 1,
		path:       path,
		fd:         fd,
	}, nil
}

func (dev *device) Close() error {
	if err := dev.Stop(); err != nil {
		return err
	}

	return unix.Close(dev.fd)
}

func (dev *device) ioctl(request uint, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(dev.fd),
		uintptr(request),
		uintptr(arg),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Query buffer parameters.
func (dev *device) queryBuffer(n uint32) (length, offset uint32, err error) {
	qb := v4l2_buffer{
		index:  n,
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
	}
	if err = dev.ioctl(VIDIOC_QUERYBUF, unsafe.Pointer(&qb)); err != nil {
		return
	}

	length = qb.length
	offset = nativeEndian.Uint32(qb.m[0:4])
	return
}

// Request specified number of kernel buffers memory-mapped to user-space.
func (dev *device) requestBuffers(n int) error {
	rb := v4l2_requestbuffers{
		count:  uint32(n),
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
	}
	return dev.ioctl(VIDIOC_REQBUFS, unsafe.Pointer(&rb))
}

func (dev *device) mapMemory() error {
	if dev.mmap != nil {
		panic("v4l2 device: memory already mapped")
	}

	if err := dev.requestBuffers(dev.numBuffers); err != nil {
		return err
	}

	length, offset, err := dev.queryBuffer(0)
	if err != nil {
		return err
	}

	dev.mmap, err = unix.Mmap(
		dev.fd,
		int64(offset),
		int(length),
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	return err
}

func (dev *device) unmapMemory() error {
	if dev.mmap != nil {
		if err := unix.Munmap(dev.mmap); err != nil {
			return err
		}
		dev.mmap = nil
	}

	return dev.requestBuffers(0)
}

func (dev *device) enqueue(index int) error {
	qbuf := v4l2_buffer{
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
		index:  uint32(index),
	}
	return dev.ioctl(VIDIOC_QBUF, unsafe.Pointer(&qbuf))
}

func (dev *device) dequeue(index int) (int, error) {
	dqbuf := v4l2_buffer{
		typ:   V4L2_BUF_TYPE_VIDEO_CAPTURE,
		index: uint32(index),
	}
	err := dev.ioctl(VIDIOC_DQBUF, unsafe.Pointer(&dqbuf))
	return int(dqbuf.bytesused), err
}

func (dev *device) enableStream() error {
	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	return dev.ioctl(VIDIOC_STREAMON, unsafe.Pointer(&typ))
}

func (dev *device) disableStream() error {
	// Disable stream (dequeues any outstanding buffers as well)
	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	return dev.ioctl(VIDIOC_STREAMOFF, unsafe.Pointer(&typ))
}

func (dev *device) setControl(class, id uint32, value int32) error {
	const numControls = 1

	ctrls := [numControls]v4l2_ext_control{
		v4l2_ext_control{
			id:   id,
			size: 0,
		},
	}
	nativeEndian.PutUint32(ctrls[0].value[:], uint32(value))

	extctrls := v4l2_ext_controls{
		ctrl_class: class,
		count:      numControls,
		controls:   unsafe.Pointer(&ctrls),
	}
	return dev.ioctl(VIDIOC_S_EXT_CTRLS, unsafe.Pointer(&extctrls))
}

func (dev *device) setCodecControl(id uint32, value int32) error {
	return dev.setControl(V4L2_CTRL_CLASS_MPEG, id, value)
}

func (dev *device) SetBitrate(bitrate int) error {
	return dev.setCodecControl(V4L2_CID_MPEG_VIDEO_BITRATE, int32(bitrate))
}

func (dev *device) SetPixelFormat(width, height, format int) error {
	pfmt := v4l2_pix_format{
		width:       uint32(width),
		height:      uint32(height),
		pixelformat: uint32(format),
		field:       V4L2_FIELD_ANY,
	}
	fmt := v4l2_format{
		typ: V4L2_BUF_TYPE_VIDEO_CAPTURE,
		fmt: pfmt.marshal(),
	}
	return dev.ioctl(VIDIOC_S_FMT, unsafe.Pointer(&fmt))
}

func (dev *device) SetRepeatSequenceHeader(on bool) error {
	var value int32
	if on {
		value = 1
	}
	return dev.setCodecControl(V4L2_CID_MPEG_VIDEO_REPEAT_SEQ_HEADER, value)
}

// Start video capture.
func (dev *device) Start() error {
	if err := dev.mapMemory(); err != nil {
		return err
	}

	for i := 0; i < dev.numBuffers; i++ {
		if err := dev.enqueue(i); err != nil {
			return err
		}
	}

	return dev.enableStream()
}

// Stop video capture.
func (dev *device) Stop() error {
	// Disable stream (dequeues any outstanding buffers as well).
	if err := dev.disableStream(); err != nil {
		return nil
	}

	return dev.unmapMemory()
}

// Read a video frame from the device. Blocks until data is available.
func (dev *device) ReadFrame() (out []byte, err error) {
	if dev.mmap == nil {
		panic("v4l2 device: illegal state, capture not started")
	}

	n, err := dev.dequeue(0)
	if err != nil {
		if err == syscall.EINVAL {
			err = io.EOF
		}
		return
	}

	// Copy data to new heap-allocated buffer.
	out = append([]byte(nil), dev.mmap[:n]...)

	err = dev.enqueue(0)
	return
}
