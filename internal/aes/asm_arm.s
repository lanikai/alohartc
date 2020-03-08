// Derived from Nettle's ARMv5 assembly implementation:
//   https://github.com/gnutls/nettle/blob/0fc4c0e/arm/aes-encrypt-internal.asm
//
// Reassigned registers to avoid R10 and R11 (reserved for the Go
// compiler/linker on ARM). But since encryptBlockAsm only operates on a single
// 16-byte block, we don't need the LENGTH parameter.
//
// See also:
//    https://en.wikipedia.org/wiki/Advanced_Encryption_Standard#High-level_description_of_the_algorithm

#include "textflag.h"

#define COUNT R2
#define KEY R9
#define SRC R1
#define DST R1

#define W0 R4
#define W1 R5
#define W2 R6
#define W3 R7

// Mask set to 0x3fc (or 0xff << 2). Storing it in a register allows us to
// combine 8-byte truncation with left-shift-by-2 into a single operation.
#define MASK R0

// Precomputed table lookup (overlaps COUNT).
#define TABLE R2

// For temporary values
#define T0 R8

#define X0 R1
#define X1 R3
#define X2 R12
#define X3 R14

// Golang assembly Rosetta Stone:
//    .P  means post-increment, so "MOVBU.P 1(SRC), W0" translates to "LDR W0, [SRC], #+1"

// push {r}
#define PUSHW(r) \
	MOVW.W	r, -4(R13)

// pop {r}
#define POPW(r) \
	MOVW.P	4(R13), r

// Loads one word, and adds it to the subkey. Uses T0.
// See https://github.com/gnutls/nettle/blob/f6360a0/arm/aes.m4
#define AES_LOAD(src, key, dst) \
	MOVBU.P	1(src), dst; \
	MOVBU.P	1(src), T0; \
	ORR	T0<<8, dst, dst; \
	MOVBU.P	1(src), T0; \
	ORR	T0<<16, dst, dst; \
	MOVBU.P	1(src), T0; \
	ORR	T0<<24, dst, dst; \
	MOVW.P	4(key), T0; \
	EOR	T0, dst, dst

// Stores one word. Destroys input.
#define AES_STORE(dst, x) \
	MOVBU.P	x, 1(dst); \
	MOVW	x@>8, x; \
	MOVBU.P	x, 1(dst); \
	MOVW	x@>8, x; \
	MOVBU.P	x, 1(dst); \
	MOVW	x@>8, x; \
	MOVBU.P	x, 1(dst)

// The mask argument should hold the constant 0xff.
#define AES_FINAL_ROUND_V5(a, b, c, d, key, res, mask) \
	AND	a, mask, T0; \
	MOVBU	T0<<0(TABLE), res; \
	AND	b@>8, mask, T0; \
	MOVBU	T0<<0(TABLE), T0; \
	EOR	T0<<8, res, res; \
	AND	c@>16, mask, T0; \
	MOVBU	T0<<0(TABLE), T0; \
	EOR	T0<<16, res, res; \
	MOVBU	d>>24(TABLE), T0; \
	EOR	T0<<24, res, res; \
	MOVW.P	4(key), T0; \
	EOR	T0, res, res

// MASK should hold the constant 0x3fc.
#define AES_ENCRYPT_ROUND(x0,x1,x2,x3,w0,w1,w2,w3,key) \
	MOVW	$·dtable(SB), TABLE; \
	AND	x0<<2, MASK, T0; \
	MOVW	T0<<0(TABLE), w0; \
	AND	x1<<2, MASK, T0; \
	MOVW	T0<<0(TABLE), w1; \
	AND	x2<<2, MASK, T0; \
	MOVW	T0<<0(TABLE), w2; \
	AND	x3<<2, MASK, T0; \
	MOVW	T0<<0(TABLE), w3; \
	\
	AND	x1@>6, MASK, T0; \
	ADD	$1024, TABLE; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w0, w0; \
	AND	x2@>6, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w1, w1; \
	AND	x3@>6, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w2, w2; \
	AND	x0@>6, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w3, w3; \
	\
	AND	x2@>14, MASK, T0; \
	ADD	$1024, TABLE; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w0, w0; \
	AND	x3@>14, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w1, w1; \
	AND	x0@>14, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w2, w2; \
	AND	x1@>14, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w3, w3; \
	\
	AND	x3@>22, MASK, T0; \
	ADD	$1024, TABLE; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w0, w0; \
	AND	x0@>22, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w1, w1; \
	AND	x1@>22, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	EOR	T0, w2, w2; \
	AND	x2@>22, MASK, T0; \
	MOVW	T0<<0(TABLE), T0; \
	\
	MOVM.IA.W	(key), [x0,x1,x2,x3]; \
	EOR	T0, w3, w3; \
	EOR	x0, w0; \
	EOR	x1, w1; \
	EOR	x2, w2; \
	EOR	x3, w3


// func encryptBlockAsm(nr int, xk *uint32, dst, src *byte)
TEXT ·encryptBlockAsm(SB), NOSPLIT, $0
	MOVW	nr+0(FP), COUNT
	MOVW	xk+4(FP), KEY
	MOVW	src+12(FP), SRC

	// Save link register R14 (overlaps with X3).
	PUSHW(R14)

	// Load 16 bytes from SRC into registers W0-W3, xor'ing with initial
	// round key.
	AES_LOAD(SRC, KEY, W0)
	AES_LOAD(SRC, KEY, W1)
	AES_LOAD(SRC, KEY, W2)
	AES_LOAD(SRC, KEY, W3)

	// Set MASK to 0xff << 2.
	MOVW	$0x3fc, MASK

	// Save COUNT on the stack (here and at the start of each new round).
	PUSHW(COUNT)

	// Perform nr-1 rounds of AES, alternately using registers W0-W3 or
	// X0-X3 for the current state.
	B	Lentry

Lround_loop:
	PUSHW(COUNT)

	// Transform X -> W
	AES_ENCRYPT_ROUND(X0, X1, X2, X3, W0, W1, W2, W3, KEY)

Lentry:
	// Transform W -> X
	AES_ENCRYPT_ROUND(W0, W1, W2, W3, X0, X1, X2, X3, KEY)

	// Load COUNT from stack and decrement.
	POPW(COUNT)
	SUB.S	$2, COUNT, COUNT

	B.NE	Lround_loop

	// Final round
	MOVW	$0xff, MASK
	MOVW	$·sbox0(SB), TABLE
	AES_FINAL_ROUND_V5(X0, X1, X2, X3, KEY, W0, MASK)
	AES_FINAL_ROUND_V5(X1, X2, X3, X0, KEY, W1, MASK)
	AES_FINAL_ROUND_V5(X2, X3, X0, X1, KEY, W2, MASK)
	AES_FINAL_ROUND_V5(X3, X0, X1, X2, KEY, W3, MASK)

	// Pop link register.
	POPW(R14)

	MOVW	dst+8(FP), DST
	AES_STORE(DST, W0)
	AES_STORE(DST, W1)
	AES_STORE(DST, W2)
	AES_STORE(DST, W3)

	RET


// func xorBytesAsm(dst, a, b *byte, n int)
TEXT ·xorBytesAsm(SB), NOSPLIT, $0
	MOVW	dst+0(FP), R1
	MOVW	a+4(FP), R2
	MOVW	b+8(FP), R3
	MOVW	n+12(FP), R4	// R4 is n, which counts down to 0

	TST	$3, R4
	B.EQ	loop_4b

loop_1b:
	// xor one byte at a time until we reach an even multiple of 4.
	MOVBU.P	1(R2), R5
	MOVBU.P	1(R3), R6
	EOR	R5, R6
	MOVBU.P	R6, 1(R1)
	
	SUB	$1, R4, R4
	TST	$3, R4
	B.NE	loop_1b

	CMP	$0, R4
	B.EQ	done

loop_4b:
	// xor four bytes at a time.
	MOVW.P	4(R2), R5
	MOVW.P	4(R3), R6
	EOR	R5, R6
	MOVW.P	R6, 4(R1)

	SUB.S	$4, R4, R4
	B.NE	loop_4b

done:
	RET
