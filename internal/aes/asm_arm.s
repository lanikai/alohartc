// ARM assembly implementation for AES encryption.
//
// Originally based on code from the Nettle library:
// (https://github.com/gnutls/nettle/blob/0fc4c0e/arm/aes-encrypt-internal.asm)
// but rewritten for better compatibility with the Go standard library's
// crypto/aes package.
//
// For a high-level description of the algorithm, see:
//    https://en.wikipedia.org/wiki/Advanced_Encryption_Standard

#include "textflag.h"

// Register assignments for encryptBlockAsm. We use all available registers,
// including R11 which is usually reserved for the Go linker.

// 16-byte AES state.
#define S0 R0
#define S1 R1
#define S2 R2
#define S3 R3

// 16-byte AES state (output).
#define T0 R4
#define T1 R5
#define T2 R6
#define T3 R7

// Loop counter (nr).
#define COUNT R8

// Precomputed round keys (xk).
#define KEY R9

// Temporary values.
#define TMP R11

// Source (src), destination (dst), and precomputed tables (sbox0, te0-te3). SRC
// is needed only at the beginning of the function, DST only at the end.
#define SRC R12
#define DST R12
#define TABLE R12

// Mask set to either 0xff or 0x3fc (0xff << 2). Storing it in a register allows
// us to combine masking (specifically 8-byte truncation) and shifting into a
// single operation. This takes advantage of the ARM instruction set's "flexible
// second operand", where Operand2 is a register with optional shift.
#define MASK R14

// Golang assembly Rosetta Stone:
//    .P  means post-increment, so "MOVBU.P 1(R4), R0" == "ldr r0, [r4], #+1"
//    ·   middle dot (U+00B7), replaces . (period) in assembly expressions
//    ∕   division slash (U+2215), replaces / (slash) in assembly expressions
// Register order is reversed compared to normal ARM assembly. In Go, data
// always flows left to right:
//    "AND R1<<2, R0, R8"  ==  "and r8, r0, r1, lsl #2"
// In particular this means that ARM's "flexible second operand" actually comes
// first.

// push {r}
#define PushWord(r) \
	MOVW.W	r, -4(R13)

// pop {r}
#define PopWord(r) \
	MOVW.P	4(R13), r

// Load one word from src and advance src pointer.
#define LoadWord(src, x) \
	MOVW.P	4(src), x

// Store one word to dst and advance dst pointer.
#define StoreWord(x, dst) \
	MOVW.P	x, 4(dst)

// Load next 4 words from key and xor with s0-s3, using t0-t3 for temporaries.
#define AddRoundKey(t0, t1, t2, t3, s0, s1, s2, s3) \
	MOVM.IA.W	(KEY), [t0,t1,t2,t3]; \
	EOR	t0, s0; \
	EOR	t1, s1; \
	EOR	t2, s2; \
	EOR	t3, s3

// Perform one AES round, reading current state from s0-s3 and writing to t0-t3.
// MASK should be set to 0x3cf.
#define InnerRound(s0, s1, s2, s3, t0, t1, t2, t3) \
	MOVW	$·dtable(SB), TABLE; \
	AND	s0<<2, MASK, TMP; \
	MOVW	TMP<<0(TABLE), t0; \
	AND	s1<<2, MASK, TMP; \
	MOVW	TMP<<0(TABLE), t1; \
	AND	s2<<2, MASK, TMP; \
	MOVW	TMP<<0(TABLE), t2; \
	AND	s3<<2, MASK, TMP; \
	MOVW	TMP<<0(TABLE), t3; \
	\
	ADD	$1024, TABLE; \
	AND	s1@>6, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t0, t0; \
	AND	s2@>6, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t1, t1; \
	AND	s3@>6, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t2, t2; \
	AND	s0@>6, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t3, t3; \
	\
	ADD	$1024, TABLE; \
	AND	s2@>14, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t0, t0; \
	AND	s3@>14, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t1, t1; \
	AND	s0@>14, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t2, t2; \
	AND	s1@>14, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t3, t3; \
	\
	ADD	$1024, TABLE; \
	AND	s3@>22, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t0, t0; \
	AND	s0@>22, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t1, t1; \
	AND	s1@>22, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t2, t2; \
	AND	s2@>22, MASK, TMP; \
	MOVW	TMP<<0(TABLE), TMP; \
	EOR	TMP, t3, t3

// Perform final AES round, reading current state from t0-t3 and writing to
// s0-s3.
#define FinalRound(t0, t1, t2, t3, s0, s1, s2, s3) \
	MOVW	$0xff, MASK; \
	MOVW	$crypto∕aes·sbox0(SB), TABLE; \
	\
	AND	t0, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), s0; \
	AND	t1>>8, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<8, s0; \
	AND	t2>>16, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<16, s0; \
	MOVBU	t3>>24(TABLE), TMP; \
	EOR	TMP<<24, s0; \
	\
	AND	t1, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), s1; \
	AND	t2>>8, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<8, s1; \
	AND	t3>>16, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<16, s1; \
	MOVBU	t0>>24(TABLE), TMP; \
	EOR	TMP<<24, s1; \
	\
	AND	t2, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), s2; \
	AND	t3>>8, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<8, s2; \
	AND	t0>>16, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<16, s2; \
	MOVBU	t1>>24(TABLE), TMP; \
	EOR	TMP<<24, s2; \
	\
	AND	t3, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), s3; \
	AND	t0>>8, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<8, s3; \
	AND	t1>>16, MASK, TMP; \
	MOVBU	TMP<<0(TABLE), TMP; \
	EOR	TMP<<16, s3; \
	MOVBU	t2>>24(TABLE), TMP; \
	EOR	TMP<<24, s3



// func encryptBlockAsm(nr int, xk *uint32, dst, src *byte)
TEXT ·encryptBlockAsm(SB), NOSPLIT, $0
	MOVW	nr+0(FP), COUNT
	MOVW	xk+4(FP), KEY
	MOVW	src+12(FP), SRC

	// Save link register R14 to the stack (overlaps with MASK).
	PushWord(R14)

	// Set MASK to 0xff << 2 == 0x3fc.
	MOVW	$0xff<<2, MASK

	// Load 16 bytes from SRC into registers S0-S3.
	LoadWord(SRC, S0)
	LoadWord(SRC, S1)
	LoadWord(SRC, S2)
	LoadWord(SRC, S3)

	// Xor with round key using registers T0-T3 for scratch.
	AddRoundKey(T0, T1, T2, T3, S0, S1, S2, S3)

	// Perform nr-1 rounds of AES, alternately using registers S0-S3 or
	// T0-T3 for the current state. We perform two rounds per loop, so start
	// in the middle since we need an odd number of rounds. (This saves a
	// few JMP instructions compared to a more natural loop counting.)
	B	Lstart

Lround2:
	// Transform T* -> S*.
	InnerRound(T0, T1, T2, T3, S0, S1, S2, S3)
	AddRoundKey(T0, T1, T2, T3, S0, S1, S2, S3)

Lstart:
	// Transform S* -> T*.
	InnerRound(S0, S1, S2, S3, T0, T1, T2, T3)
	AddRoundKey(S0, S1, S2, S3, T0, T1, T2, T3)

	SUB.S	$2, COUNT, COUNT
	B.NE	Lround2

	FinalRound(T0, T1, T2, T3, S0, S1, S2, S3)
	AddRoundKey(T0, T1, T2, T3, S0, S1, S2, S3)

	// Pop link register.
	PopWord(R14)

	// Store 16 bytes back to DST.
	MOVW	dst+8(FP), DST
	StoreWord(S0, DST)
	StoreWord(S1, DST)
	StoreWord(S2, DST)
	StoreWord(S3, DST)

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
