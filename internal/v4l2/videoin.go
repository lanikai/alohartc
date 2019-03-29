// Video4Linux is a Linux-specific API. Only build if GOOS=linux.
// +build linux

package v4l2

import (
	"encoding/binary"
	"io"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type VideoReader struct {
	io.Closer
	io.Reader

	numBuffers int // Number of requested kernel driver buffers

	fd   int    // File descriptor
	data []byte // Memory-mapped buffer
}

// Close video device
func (r *VideoReader) Close() error {
	if err := r.Stop(); err != nil {
		return err
	}

	return unix.Close(r.fd)
}

// Flip video horizontally
func (r *VideoReader) FlipHorizontal() error {
	ctrl := v4l2_control{V4L2_CID_HFLIP, 1}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_S_CTRL),
		uintptr(unsafe.Pointer(&ctrl)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Flip video vertically
func (r *VideoReader) FlipVertical() error {
	ctrl := v4l2_control{V4L2_CID_VFLIP, 1}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_S_CTRL),
		uintptr(unsafe.Pointer(&ctrl)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// Open specified video device
func Open(name string, config *Config) (*VideoReader, error) {
	var err error

	r := &VideoReader{
		numBuffers: 1,
	}

	// Open device
	r.fd, err = unix.Open(name, unix.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	// Set pixel format
	if err = r.setPixelFormat(uint32(config.Width), uint32(config.Height), uint32(config.Format)); err != nil {
		return r, err
	}

	// Set repeat sequence headers
	var repeatSequenceHeader int32
	if config.RepeatSequenceHeader {
		repeatSequenceHeader = 1
	}
	if err = r.SetCodecControl(V4L2_CID_MPEG_VIDEO_REPEAT_SEQ_HEADER, repeatSequenceHeader); err != nil {
		return r, err
	}

	return r, nil
}

// Read data from video device. Blocks until data available.
func (r *VideoReader) Read(p []byte) (int, error) {
	// Dequeue memory-mapped buffer
	if n, errno := r.dequeue(); errno != 0 {
		if errno == syscall.EINVAL {
			return n, io.EOF
		} else {
			return n, errno
		}
	} else {
		// Copy data from memory-mapped buffer
		copy(p, r.data[:n])

		// Re-enqueue memory-mapped buffer
		r.enqueue(0)

		return n, nil
	}
}

// Set bitrate
func (r *VideoReader) SetBitrate(bitrate uint) error {
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
		return errno
	}
	return nil
}

// Dequeue buffer from device out-buffer queue
func (r *VideoReader) dequeue() (int, syscall.Errno) {
	dqbuf := v4l2_buffer{
		typ: V4L2_BUF_TYPE_VIDEO_CAPTURE,
	}
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_DQBUF),
		uintptr(unsafe.Pointer(&dqbuf)),
	)
	return int(dqbuf.bytesused), errno
}

// Start video capture
func (r *VideoReader) Start() error {
	var err error

	// Request buffers
	if err = r.requestBuffers(r.numBuffers); err != nil {
		return err
	}

	// Query buffer
	if length, offset, err := r.queryBuffer(0); err != nil {
		return err
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
			return err
		}
	}

	// Enqueue buffers
	for i := 0; i < r.numBuffers; i++ {
		if err = r.enqueue(i); err != nil {
			return err
		}
	}

	// Enable stream
	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_STREAMON),
		uintptr(unsafe.Pointer(&typ)),
	)
	if errno != 0 {
		return errno
	}

	return nil
}

// Stop video capture
func (r *VideoReader) Stop() error {
	// Disable stream (dequeues any outstanding buffers as well)
	typ := V4L2_BUF_TYPE_VIDEO_CAPTURE
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(r.fd),
		uintptr(VIDIOC_STREAMOFF),
		uintptr(unsafe.Pointer(&typ)),
	)
	if errno != 0 {
		return errno
	}

	// Unmap memory
	if err := unix.Munmap(r.data); err != nil {
		return err
	}

	// Deallocate buffers
	if err := r.requestBuffers(0); err != nil {
		return err
	}

	return nil
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
		return errno
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
		return 0, 0, errno
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
		return errno
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
		return errno
	}

	return nil
}
