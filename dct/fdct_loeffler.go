// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dct

func fdctLoeffler(b *block) {
	fdctLoefflerCols(b)
	fdctLoefflerRows(b)
}

func box4i(x0, x1 int32, kcos, ksin int32, shift uint) (int32, int32) {
	a := kcos
	b := ksin
	ax := a * (x0 + x1)
	y0 := ((b-a)*(x1) + ax) >> shift
	y1 := (-(a+b)*(x0) + ax) >> shift
	return (y0), (y1)
}

func fdctLoefflerCols(b *block) {
	const (
		cos1_16 = 64277 // 16 fix cos 1*pi/16
		sin1_16 = 12785 // 16 fix sin 1*pi/16
		sin1_18 = 51142
		cos1_18 = 257107
		cos1_22 = 4113712 // 22 fix cos 1*pi/16
		sin1_22 = 818268  // 22 fix sin 1*pi/16
		cos3_16 = 54491   // 16 fix cos 3*pi/16
		sin3_16 = 36410   // 16 fix sin 3*pi/16
		cos3_22 = 3487436 // 22 fix cos 3*pi/16
		cos3_18 = 217965
		sin3_22 = 2330230 // 22 fix sin 3*pi/16
		sin3_18 = 145639

		sqrt2cos6_16 = 35468   // 16 fix (sqrt 2)*cos 6*pi/16
		sqrt2cos6_18 = 141871  // 18 fix (sqrt 2)*cos 6*pi/16
		sqrt2cos6_21 = 1134970 // 21 fix (sqrt 2)*cos 6*pi/16  (18 bits)
		sqrt2sin6_16 = 85627   // 16 fix (sqrt 2)*sin 6*pi/16
		sqrt2sin6_18 = 342508  // 18 fix (sqrt 2)*sin 6*pi/16
		sqrt2sin6_21 = 2740061 // 21 fix (sqrt 2)*sin 6*pi/16  (19 bits)

		sqrt2_16 = 92682   // 16 fix sqrt 2
		sqrt2_18 = 370728  // 18 fix sqrt 2 (19 bits)
		sqrt2_22 = 5931642 // 22 fix  sqrt 2 (23 bits)

		cos1_32      = 4212440704 // 32 fix cos 1*pi/16
		sin1_32      = 837906553  // 32 fix sin 1*pi/16
		cos3_32      = 3571134792 // 32 fix cos 3*pi/16
		sin3_32      = 2386155981 // 32 fix sin 3*pi/16
		sqrt2cos6_32 = 2324419551 // 32 fix (sqrt 2)*cos 6*pi/16
		sqrt2sin6_32 = 5611645204 // 32 fix (sqrt 2)*sin 6*pi/16
		sqrt2_32     = 6074001000 // 32 fix sqrt 2
	)

	for i := range 8 {
		x0 := b[0*8+i]
		x1 := b[1*8+i]
		x2 := b[2*8+i]
		x3 := b[3*8+i]
		x4 := b[4*8+i]
		x5 := b[5*8+i]
		x6 := b[6*8+i]
		x7 := b[7*8+i]

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4

		x4, x7 = box4i(x4, x7, cos3_18, sin3_18, 0)
		x5, x6 = box4i(x5, x6, cos1_18, sin1_18, 0)

		x0, x3 = x0+x3, x0-x3
		x1, x2 = x1+x2, x1-x2
		x2, x3 = box4i(x2, x3, sqrt2cos6_18, sqrt2sin6_18, 0)

		x0, x1 = x0+x1, x0-x1
		b[0*8+i] = (x0 - 128*8) << 18
		b[4*8+i] = x1 << 18

		x4, x6 = x4+x6, x4-x6
		x7, x5 = x7+x5, x7-x5
		x7, x4 = x7+x4, x7-x4

		x6 = int32((int32(x6>>16) * sqrt2_16) >> 0)
		x5 = int32((int32(x5>>16) * sqrt2_16) >> 0)

		b[1*8+i] = x7
		b[2*8+i] = x2
		b[3*8+i] = x5
		b[5*8+i] = x6
		b[6*8+i] = x3
		b[7*8+i] = x4
	}
}

func fdctLoefflerRows(b *block) {
	const (
		cos1_16 = 64277  // 16 fix cos 1*pi/16
		sin1_16 = 12785  // 16 fix sin 1*pi/16
		cos1_19 = 514214 // 19 fix cos 1*pi/16
		sin1_19 = 102284 // 19 fix sin 1*pi/16
		cos3_16 = 54491  // 16 fix cos 3*pi/16
		sin3_16 = 36410  // 16 fix sin 3*pi/16
		cos3_19 = 435930 // 19 fix cos 3*pi/16
		sin3_19 = 291279 // 19 fix sin 3*pi/16

		sqrt2cos6_16 = 35468  // 16 fix (sqrt 2)*cos 6*pi/16
		sqrt2cos6_18 = 141871 // 18 fix (sqrt 2)*cos 6*pi/16  (18 bits)
		sqrt2sin6_16 = 85627  // 16 fix (sqrt 2)*sin 6*pi/16
		sqrt2sin6_18 = 342508 // 18 fix (sqrt 2)*sin 6*pi/16  (19 bits)

		sqrt2_16 = 92682  // 16 fix sqrt 2
		sqrt2_18 = 370728 // 18 fix sqrt 2 (19 bits)

		cos1_32      = 4212440704 // 32 fix cos 1*pi/16
		sin1_32      = 837906553  // 32 fix sin 1*pi/16
		cos3_32      = 3571134792 // 32 fix cos 3*pi/16
		sin3_32      = 2386155981 // 32 fix sin 3*pi/16
		sqrt2cos6_32 = 2324419551 // 32 fix (sqrt 2)*cos 6*pi/16
		sqrt2sin6_32 = 5611645204 // 32 fix (sqrt 2)*sin 6*pi/16
		sqrt2_32     = 6074001000 // 32 fix sqrt 2

		cos1 = 0.980785280403
		sin1 = 0.195090322016
	)

	for i := range 8 {
		x := b[8*i : 8*i+8 : 8*i+8]
		x0, x1, x2, x3, x4, x5, x6, x7 := x[0], x[1], x[2], x[3], x[4], x[5], x[6], x[7]

		x0, x7 = x0+x7, x0-x7
		x1, x6 = x1+x6, x1-x6
		x2, x5 = x2+x5, x2-x5
		x3, x4 = x3+x4, x3-x4

		x0, x3 = x0+x3, x0-x3
		x1, x2 = x1+x2, x1-x2

		x0, x1 = x0+x1, x0-x1

		x2, x3 = box4i(x2>>16, x3>>16, sqrt2cos6_16, sqrt2sin6_16, 0)
		x4, x7 = box4i(x4>>16, x7>>16, cos3_16, sin3_16, 0)
		x5, x6 = box4i(x5>>16, x6>>16, cos1_16, sin1_16, 0)

		x4, x6 = x4+x6, x4-x6
		x7, x5 = x7+x5, x7-x5
		x7, x4 = x7+x4, x7-x4

		x6 = int32((int32(x6>>16) * sqrt2_16) >> 0)
		x5 = int32((int32(x5>>16) * sqrt2_16) >> 0)

		x[0] = x0 >> 17
		x[1] = x7 >> 17
		x[2] = x2 >> 17
		x[3] = x5 >> 17
		x[4] = x1 >> 17
		x[5] = x6 >> 17
		x[6] = x3 >> 17
		x[7] = x4 >> 17
	}
}
