// Video4Linux is a Linux-specific API. Only build if GOOS=linux.
// +build linux

package v4l2

import (
	"encoding/binary"
	"io"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type VideoReader struct {
	io.Closer
	io.Reader

	width  uint // Video width in pixels
	height uint // Video height in pixels

	numBuffers int // Number of requested kernel driver buffers

	fd   int    // File descriptor
	data []byte // Memory-mapped buffer

	active bool
}

// Close video device
func (r *VideoReader) Close() error {
	if err := r.Stop(); err != nil {
		return err
	}
	if err := unix.Munmap(r.data); err != nil {
		return err
	}

	return unix.Close(r.fd)
}

// Open H.264 encoded bitstream from specified device
func OpenH264(name string, width, height uint) (*VideoReader, error) {
	return Open(name, width, height, V4L2_PIX_FMT_H264)
}

// Open specified video device
func Open(name string, width, height, format uint) (*VideoReader, error) {
	var err error

	r := &VideoReader{
		width:      width,
		height:     height,
		numBuffers: 1,
	}

	// Open device
	r.fd, err = unix.Open(name, unix.O_RDWR|unix.O_NONBLOCK, 0666)
	if err != nil {
		return nil, err
	}

	// Set pixel format
	if err = r.setPixelFormat(uint32(width), uint32(height), uint32(format)); err != nil {
		return r, err
	}

	// Request buffers
	if err = r.requestBuffers(r.numBuffers); err != nil {
		return r, err
	}

	// Query buffer
	if length, offset, err := r.queryBuffer(0); err != nil {
		return r, err
	} else {
		// Memory map
		r.data, err = unix.Mmap(
			r.fd,
			int64(offset),
			int(length),
			unix.PROT_READ|unix.PROT_WRITE,
			unix.MAP_SHARED,
		)
		if err != nil {
			return r, err
		}
	}

	// Enqueue buffers
	for i := 0; i < r.numBuffers; i++ {
		if err = r.enqueue(i); err != nil {
			return r, err
		}
	}

	return r, nil
}

// Read data from video device. Blocks until data available.
func (r *VideoReader) Read(p []byte) (int, error) {
	fds := unix.FdSet{}

	// Set bit in set corresponding to file descriptor
	fds.Bits[r.fd>>6] |= 1 << (uint(r.fd) & 0x3F)

	// Block until data available
	_, err := unix.Select(r.fd+1, &fds, nil, nil, nil)
	if err != nil {
		return 0, err
	}

	// Dequeue memory-mapped buffer
	n, err := r.dequeue()
	if err != nil {
		return n, err
	}

	// Copy data from memory-mapped buffer
	copy(p, r.data[:n])

	// Re-enqueue memory-mapped buffer
	r.enqueue(0)

	return n, nil
}

// Set bitrate
func (r *VideoReader) SetBitrate(bitrate int) error {
	return r.SetCodecControl(V4L2_CID_MPEG_VIDEO_BITRATE, int32(bitrate))
}

// Set codec control
func (r *VideoReader) SetCodecControl(id uint32, value int32) error {
	return r.SetControl(V4L2_CTRL_CLASS_MPEG, id, value)
}

// Set control
func (r *VideoReader) SetControl(class, id uint32, value int32) error {
	const numControls = 1

	ctrls := [numControls]v4l2_ext_control{
		v4l2_ext_control{
			id:   id,
			size: 0,
		},
	}

	switch nativeEndian {
	case binary.LittleEndian:
		binary.LittleEndian.PutUint32(ctrls[0].value[:], uint32(value))
	case binary.BigEndian:
		binary.BigEndian.PutUint32(ctrls[0].value[:], uint32(value))
	default:
		panic("nativeEndian is not set")
	}

	extctrls := v4l2_ext_controls{
		ctrl_class: class,
		count:      numControls,
		controls:   unsafe.Pointer(&ctrls),
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_S_EXT_CTRLS),
		uintptr(unsafe.Pointer(&extctrls)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}
	return nil
}

// Dequeue buffer from device out-buffer queue
func (r *VideoReader) dequeue() (int, error) {
	dqbuf := v4l2_buffer{
		typ: V4L2_BUF_TYPE_VIDEO_CAPTURE,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_DQBUF),
		uintptr(unsafe.Pointer(&dqbuf)),
	)
	if errno != 0 {
		return int(dqbuf.bytesused), errors.New(errno.Error())
	}
	return int(dqbuf.bytesused), nil
}

// Enqueue buffer into device in-buffer queue
func (r *VideoReader) enqueue(index int) error {
	qbuf := v4l2_buffer{
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
		index:  uint32(index),
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_QBUF),
		uintptr(unsafe.Pointer(&qbuf)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}
	return nil
}

// Query buffer parameters
func (r *VideoReader) queryBuffer(n uint32) (length, offset uint32, err error) {
	qb := v4l2_buffer{
		index:  n,
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_QUERYBUF),
		uintptr(unsafe.Pointer(&qb)),
	)
	if errno != 0 {
		return 0, 0, errors.New(errno.Error())
	}

	switch nativeEndian {
	case binary.LittleEndian:
		offset = binary.LittleEndian.Uint32(qb.m[0:])
	case binary.BigEndian:
		offset = binary.BigEndian.Uint32(qb.m[0:])
	default:
		panic("nativeEndian is not set")
	}

	return qb.length, offset, nil
}

// Request specified number of kernel buffers memory-mapped to user-space
func (r *VideoReader) requestBuffers(n int) error {
	rb := v4l2_requestbuffers{
		count:  uint32(n),
		typ:    V4L2_BUF_TYPE_VIDEO_CAPTURE,
		memory: V4L2_MEMORY_MMAP,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_REQBUFS),
		uintptr(unsafe.Pointer(&rb)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}

	return nil
}

// Set pixel format
func (r *VideoReader) setPixelFormat(width, height, format uint32) error {
	pfmt := v4l2_pix_format{
		width:       width,
		height:      height,
		pixelformat: format,
		field:       V4L2_FIELD_ANY,
	}
	fmt := v4l2_format{
		typ: V4L2_BUF_TYPE_VIDEO_CAPTURE,
		fmt: pfmt.marshal(),
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_S_FMT),
		uintptr(unsafe.Pointer(&fmt)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}

	return nil
}

// Start video capture
func (r *VideoReader) Start() error {
	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_STREAMON),
		uintptr(unsafe.Pointer(&typ)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}

	r.active = true
	return nil
}

// Stop video capture
func (r *VideoReader) Stop() error {
	if !r.active {
		return nil
	}

	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_STREAMOFF),
		uintptr(unsafe.Pointer(&typ)),
	)
	if errno != 0 {
		return errors.New(errno.Error())
	}
	return nil
}
