// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dct

// Based on C++ code written by Pascal Massimino and Dean Gaudet at Google.

// Cosine table
//
//	C(k) = cos(π·k/16)/√2 for k=1..7, signed 15-bit precision.
//	cosine04 is
//
// In Ivy:
//
//	op C x = (cos 1/16 * pi * x) / sqrt 2
//	op bits fix x = floor 0.5 + (2**bits) * x
//	15 fix C 1 2 3 4 5 6 7
//		22725 21407 19266 16384 12873 8867 4520
//	15 fix 2 * (C 1) * C 1 2 3 4 5 6 7
//		31521 29692 26722 22725 17855 12299 6270
//	15 fix 2 * (C 2) * C 1 2 3 4 5 6 7
//		29692 27969 25172 21407 16819 11585 5906
//	15 fix 2 * (C 3) * C 1 2 3 4 5 6 7
//		26722 25172 22654 19266 15137 10426 5315
var (
	cosine04 = [7]int32{22725, 21407, 19266, 16384, 12873, 8867, 4520}
	cosine17 = [7]int32{31521, 29692, 26722, 22725, 17855, 12299, 6270}
	cosine26 = [7]int32{29692, 27969, 25172, 21407, 16819, 11585, 5906}
	cosine35 = [7]int32{26722, 25172, 22654, 19266, 15137, 10426, 5315}

	cosine = [8]*[7]int32{
		&cosine04,
		&cosine17,
		&cosine26,
		&cosine35,
		&cosine04,
		&cosine35,
		&cosine26,
		&cosine17,
	}
)

const (
	tan1_16 = 13036  // 16 fix tan pi/16 (only 14 unsigned bits)
	tan1_18 = 52144  // 18 fix tan pi/16 (only 16 unsigned bits)
	tan1_20 = 208575 //  20 fix tan pi/16 (only 18 unsigned bits)

	tan2_16 = 27147   // 16 fix tan 2*pi/16 (only 15 unsigned bits)
	tan2_19 = 217167  // 19 fix tan 2*pi/16 (only 18 unsigned bits)
	tan2_22 = 1737338 // 22 fix tan 2*pi/16 (only 21 unsigned bits)
	tan2_23 = 3474675 // 23 fix tan 2*pi/16 (only 22 unsigned bits)

	tan3_16     = 43790   // 16 fix tan 3*pi/16
	tan3_18     = 175159  // 18 fix tan 3*pi/16
	kTan1       = 13036   // = tan(pi/16)
	kTan2       = 27146   // = tan(2pi/16) = sqrt(2) - 1.
	kTan3m1     = -21746  // = tan(3pi/16) - 1
	k2Sqrt2     = 23170   // 16 fix 1/(2*sqrt 2) (only 15 unsigned bits)
	invSqrt2_15 = 23170   // 15 fix 1/sqrt 2
	invSqrt2_16 = 46341   // 16 fix 1/sqrt 2
	invSqrt2_19 = 370728  // 19 fix 1/sqrt 2
	invSqrt2_22 = 2965821 // 22 fix 1/sqrt 2
)

// Constants for IDCT horizontal pass.
//
// These rounding constants not only incorporate the 1 << (ROW_SHIFT-1)
// you would expect for correctly rounding during RowIdct()'s descaling,
// but also prepare for a 2nd-order rounding during the vertical pass.
// In ColumnIdct, we perform 16bit multiply using MULT(x,K) = (x * K) >> 16
// But we must do the proper rounding (x * K + (1<<15)) >> 16 to achieve
// the correct precision. This would mean storing the intermediate 31bit
// result x*K, adding the 1<<15 rounding constant, and descaling.
// So, the trick, since K is a constant, is to pre-condition x by computing
// x' = x + (1<<15)/K instead. This means adding an additional rounding
// constant R' = (1<<15)/K, so that x' * K is approximatively equal
// to (x * K + (1<<15)) already. We can then proceed with keeping the
// upper 15bits of this result, which is of greater precision.
// This trick was originally devised by Michel Lespinasse.
// The constants depend obviously of the algorithm used for the 2nd pass,
// and the code paths that lead to the first multiplies.
// People have used these precise values extensively, even if they could be
// refined a little further. To prevent mild idct mismatch with material
// encoded using this trick, we are better using these rounding constants
// 'as is' too. Note that they can be expressed in closed form, albeit it's
// a little complex. The simplest ones are:
//
//	kRound2 = .5 + cos(pi/8)/sqrt(2) * cos(pi/8)
//	kRound6 = .5 - cos(pi/8)/sqrt(2) * sin(pi/8)
const (
	kRound0 = 65536 // 1 << (COL_SHIFT + ROW_SHIFT - 1);
	kRound1 = 3597  // FIX(.5 + 1.25683487303)
	kRound2 = 2260  // FIX(.5 + 0.60355339059)
	kRound3 = 1203  // FIX(.5 + 0.087788325588)
	kRound4 = 0     // FIX(.5 - 0.5)
	kRound5 = 120   // FIX(.5 - 0.441341716183);
	kRound6 = 512   // FIX(.5 - 0.25);
	kRound7 = 512   // FIX(.5 - 0.25);
)

const (
	rowShift = 11
	colShift = 6
)

func idctCol8(b *block) {
	for i := range 8 {
		m0 := int32(kTan3m1)
		m3 := b[3*8+i]
		m1 := m0
		m5 := b[5*8+i]

		m0 = (m0 * m3) >> 16
		m1 = (m1 * m5) >> 16
		m0 += m3
		m1 += m5
		m0 -= m5
		m1 += m3

		m4 := int32(kTan1)
		m6 := b[1*8+i]
		m2 := m4
		m7 := b[7*8+i]

		m4 = (m4 * m7) >> 16
		m2 = (m2 * m6) >> 16
		m4 += m6
		m2 -= m7

		m4, m1 = m4-m1, m4+m1
		m2, m0 = m2-m0, m2+m0
		m4, m0 = m4-m0, m4+m0

		m3 = int32(k2Sqrt2)
		m4 = (m4 * m3) >> 16
		m0 = (m0 * m3) >> 16
		m4 += m4
		m0 += m0

		m7 = int32(kTan2)
		m3 = b[2*8+i]
		m6 = b[6*8+i]
		m5 = m7
		m7 = (m7 * m6) >> 16
		m5 = (m5 * m3) >> 16
		m7 += m3
		m5 -= m6

		m3 = b[0*8+i]
		m6 = b[4*8+i]
		m3, m6 = m3-m6, m3+m6

		m3, m5 = m3-m5, m3+m5
		m6, m7 = m6-m7, m6+m7
		m5, m0 = m5-m0, m5+m0
		m3, m4 = m3-m4, m3+m4
		m7, m1 = m7-m1, m7+m1
		m6, m2 = m6-m2, m6+m2

		b[0*8+i] = m1 >> colShift
		b[1*8+i] = m0 >> colShift
		b[2*8+i] = m4 >> colShift
		b[3*8+i] = m2 >> colShift
		b[4*8+i] = m6 >> colShift
		b[5*8+i] = m3 >> colShift
		b[6*8+i] = m5 >> colShift
		b[7*8+i] = m7 >> colShift
	}
}

func idctCol4(b *block) {
	idctCol8(b)
}

func idctCol3(b *block) {
	idctCol8(b)
}

func idctRow(in *[8]int32, table *[7]int32, round int32) int32 {
	C1 := int32(table[0])
	C2 := int32(table[1])
	C3 := int32(table[2])
	C4 := int32(table[3])
	C5 := int32(table[4])
	C6 := int32(table[5])
	C7 := int32(table[6])

	right := in[5] | in[6] | in[7]
	left := in[1] | in[2] | in[3]
	if in[4]|right == 0 {
		K := C4*in[0] + round
		if left != 0 {
			a0 := K + C2*in[2]
			a3 := K - C2*in[2]
			a1 := K + C6*in[2]
			a2 := K - C6*in[2]

			b0 := C1*in[1] + C3*in[3]
			b1 := C3*in[1] - C7*in[3]
			b2 := C5*in[1] - C1*in[3]
			b3 := C7*in[1] - C5*in[3]

			in[0] = (a0 + b0) >> rowShift
			in[1] = (a1 + b1) >> rowShift
			in[2] = (a2 + b2) >> rowShift
			in[3] = (a3 + b3) >> rowShift
			in[4] = (a3 - b3) >> rowShift
			in[5] = (a2 - b2) >> rowShift
			in[6] = (a1 - b1) >> rowShift
			in[7] = (a0 - b0) >> rowShift
		} else {
			a0 := K >> rowShift
			in[0] = a0
			in[1] = a0
			in[2] = a0
			in[3] = a0
			in[4] = a0
			in[5] = a0
			in[6] = a0
			in[7] = a0
			return a0
		}
	} else if left|right == 0 {
		a0 := (round + C4*(in[0]+in[4])) >> rowShift
		a1 := (round + C4*(in[0]-in[4])) >> rowShift
		in[0] = a0
		in[1] = a1
		in[2] = a1
		in[3] = a0
		in[4] = a0
		in[5] = a1
		in[6] = a1
		in[7] = a0
	} else {
		K := C4*in[0] + round
		t0 := C4 * in[4]
		t1 := C2*in[2] + C6*in[6]
		a0 := K + t0 + t1
		a3 := K + t0 - t1
		t1 = C6*in[2] - C2*in[6]
		a1 := K - t0 + t1
		a2 := K - t0 - t1

		b0 := C1*in[1] + C3*in[3] + C5*in[5] + C7*in[7]
		b1 := C3*in[1] - C7*in[3] - C1*in[5] - C5*in[7]
		b2 := C5*in[1] - C1*in[3] + C7*in[5] + C3*in[7]
		b3 := C7*in[1] - C5*in[3] + C3*in[5] - C1*in[7]

		in[0] = (a0 + b0) >> rowShift
		in[1] = (a1 + b1) >> rowShift
		in[2] = (a2 + b2) >> rowShift
		in[3] = (a3 + b3) >> rowShift
		in[4] = (a3 - b3) >> rowShift
		in[5] = (a2 - b2) >> rowShift
		in[6] = (a1 - b1) >> rowShift
		in[7] = (a0 - b0) >> rowShift
	}
	return 1
}

func idctSkal(in *block) {
	// the first 3 rows are never zero because of the rounding
	// constants required for 2nd pass, even if in[] is zero.
	idctRow((*[8]int32)(in[0*8:]), &cosine04, kRound0)
	idctRow((*[8]int32)(in[1*8:]), &cosine17, kRound1)
	idctRow((*[8]int32)(in[2*8:]), &cosine26, kRound2)
	row3 := idctRow((*[8]int32)(in[3*8:]), &cosine35, kRound3)
	row4567 := idctRow((*[8]int32)(in[4*8:]), &cosine04, kRound4)
	row4567 |= idctRow((*[8]int32)(in[5*8:]), &cosine35, kRound5)
	row4567 |= idctRow((*[8]int32)(in[6*8:]), &cosine26, kRound6)
	row4567 |= idctRow((*[8]int32)(in[7*8:]), &cosine17, kRound7)

	if row4567 == 0 {
		if row3 == 0 {
			idctCol3(in)
		} else {
			idctCol4(in)
		}
	} else {
		idctCol8(in)
	}
}
