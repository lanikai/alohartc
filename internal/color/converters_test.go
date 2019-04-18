package color

import (
	"image"
	"testing"
)

func TestYUYVToYUV420P(t *testing.T) {
	r := image.Rect(0, 0, 1280, 720)

	yuyv := NewYUYV(r)
	yuv420p := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)

	// Write some sample data
	for i := 0; i < 2*1280*720; i++ {
		yuyv.Packed[i] = byte(i)
	}

	YUYVToYUV420P(yuv420p, yuyv)

	// Verify luma
	for i := 0; i < 1280*720; i++ {
		if yuv420p.Y[i] != byte(2*i) {
			t.FailNow()
		}
	}

	// Verify chroma
	for row := 0; row < 720/2; row++ {
		for col := 0; col < 1280/2; col++ {
			if yuv420p.Cb[1280/2*row+col] != byte(4*1280*row+4*col+1) {
				t.FailNow()
			}
			if yuv420p.Cr[1280/2*row+col] != byte(4*1280*row+4*col+3) {
				t.FailNow()
			}
		}
	}
}

func BenchmarkYUYVToYUV420PAt720P(b *testing.B) {
	r := image.Rect(0, 0, 1280, 720)
	yuyv := NewYUYV(r)
	yuv420p := image.NewYCbCr(r, image.YCbCrSubsampleRatio420)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		YUYVToYUV420P(yuv420p, yuyv)
	}
}
