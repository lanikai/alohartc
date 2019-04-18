// Copyright 2019 Lanikai Labs. All rights reserved.

package color

// #include "yuyv_to_yuv420p.h"
import "C"

import (
	"image"
	"unsafe"
)

type YUYV struct {
	Packed []uint8
	Rect   image.Rectangle
	Stride int
}

// NewYUYV allocates and returns a YUYV image
func NewYUYV(r image.Rectangle) *YUYV {
	return &YUYV{
		Packed: make([]byte, 2*r.Dx()*r.Dy()),
		Rect:   r,
		Stride: 2 * r.Dx(),
	}
}

// YUYVToYUV420 converts YUYV (i.e. YUY2) packed to YUV420 planar format
func YUYVToYUV420P(dst *image.YCbCr, src *YUYV) {
	C.yuyv_to_yuv420p(
		(*C.uchar)(unsafe.Pointer(&dst.Y[0])),
		(*C.uchar)(unsafe.Pointer(&dst.Cb[0])),
		(*C.uchar)(unsafe.Pointer(&dst.Cr[0])),
		(*C.uchar)(unsafe.Pointer(&src.Packed[0])),
		C.int(src.Stride),
		C.int(src.Rect.Dy()),
	)
}
