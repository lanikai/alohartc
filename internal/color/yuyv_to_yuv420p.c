//////////////////////////////////////////////////////////////////////////////
//
// YUYV packed to YUV420 planar conversion
//
// Copyright 2019 Lanikai Labs. All rights reserved.
//
//////////////////////////////////////////////////////////////////////////////

#if defined(__ARM_NEON)
#include <arm_neon.h>
#endif

#include "yuyv_to_yuv420p.h"


//////////////////////////////////////////////////////////////////////////////
//
// Unpack an even row. Even rows contain both luma and chroma.
//
// y, u, v: Pointers to planar destination buffers for luma and chroma.
// yuyv:    Pointer to packed source buffer.
// stride:  Stride (in bytes) of source buffer.
//
//////////////////////////////////////////////////////////////////////////////

inline static void
unpack_even(uint8_t *y, uint8_t *u, uint8_t *v, uint8_t *yuyv, int stride) {
#if defined(__ARM_NEON)
	for (int i = 0; i < stride; i += 32) {
		// arm neon: de-interleave luma and chroma
		uint8x16x2_t y_uv = vld2q_u8(yuyv);
		vst1q_u8(y, y_uv.val[0]);

		// arm neon: de-interleave chroma
		uint8x8x2_t u_v = vld2_u8((uint8_t *)(&y_uv.val[1]));
		vst1_u8(u, u_v.val[0]);
		vst1_u8(v, u_v.val[1]);

		// Increment pointers to next block
		yuyv += 32;
		y    += 16;
		u    += 8;
		v    += 8;
	}
#else
	for (int i = 0; i < stride; i += 4) {
		*y++ = *yuyv++;
		*u++ = *yuyv++;
		*y++ = *yuyv++;
		*v++ = *yuyv++;
	}
#endif
}


//////////////////////////////////////////////////////////////////////////////
//
// Unpack an odd row. Odd rows contain only luma.
//
// Arguments:
// y:      Pointer to planar destination buffer for luma.
// yuyv:   Pointer to packed source buffer.
// stride: Stride (in bytes) of source buffer.
//
//////////////////////////////////////////////////////////////////////////////

inline static void unpack_odd(uint8_t *y, uint8_t *yuyv, int stride) {
#if defined(__ARM_NEON)
	for (int i = 0; i < stride; i += 32) {
		vst1q_u8(y, vld2q_u8(yuyv).val[0]);

		yuyv += 32;
		y    += 16;
	}
#else
	for (int i = 0; i < stride; i += 4) {
		*y++ = *yuyv++;
		yuyv++;
		*y++ = *yuyv++;
		yuyv++;
	}
#endif
}


//////////////////////////////////////////////////////////////////////////////
//
// Convert YUYV to YUV420P
//
// YUYV is a packed format, where luma and chroma are interleaved, 8-bits per
// pixel:
//
//     YUYVYUYV...
//     YUYVYUYV...
//     ...
//
// Color is subsampled horizontally.
//
//
// YUV420 is a planar format, and the most common H.264 colorspace. For each
// 2x2 square of pixels, there are 4 luma values and 2 chroma values. Each
// value is 8-bits; however, there are 4*8 + 8 + 8 = 48 bits total for 4
// pixels, so on average there are effectively 12-bits per pixel:
//
// YYYY...	U.U..	V.V..
// YYYY...	.....	.....
// YYYY...	U.U..	V.V..
// YYYY...	.....	.....
// .......	.....	.....
//
// Arguments:
// y:      Pointer to planar destination buffer for luma.
// yuyv:   Pointer to packed source buffer.
// stride: Stride (in bytes) of source buffer.
//
//////////////////////////////////////////////////////////////////////////////

void yuyv_to_yuv420p(
	uint8_t *y, uint8_t *u, uint8_t *v,
	uint8_t *yuyv,
	int stride, int height
) {
	for (int row = 0; row < height; row += 2) {
		unpack_even(y, u, v, yuyv, stride);
		y    += stride / 2;
		u    += stride / 4;
		v    += stride / 4;
		yuyv += stride;

		unpack_odd(y, yuyv, stride);
		y    += stride / 2;
		yuyv += stride;
	}
}
