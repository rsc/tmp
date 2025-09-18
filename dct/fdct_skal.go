// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dct

func fdctSkal(b *block) {
	fdctSkalCols(b)
	fdctSkalRows(b)
}

func fdctSkalCols(b *block) {
	//fmt.Printf("COLS %x\n", *b)
	for i := range 8 {
		m0 := b[0*8+i]
		m1 := b[1*8+i]
		m2 := b[2*8+i]
		m3 := b[3*8+i]
		m4 := b[4*8+i]
		m5 := b[5*8+i]
		m6 := b[6*8+i]
		m7 := b[7*8+i]
		// m0..m7 now fit in 8 bits (including sign)

		m0, m7 = m0-m7, m0+m7
		m1, m6 = m1-m6, m1+m6
		m2, m5 = m2-m5, m2+m5
		m3, m4 = m3-m4, m3+m4
		// m0..m7 now fit in 9 bits

		m7, m4 = m7-m4, m7+m4
		m6, m5 = m6-m5, m6+m5
		// m4..m7 now fit in 10 bits

		m4, m5 = m4-m5, m4+m5
		// m4..m5 now fit in 11 bits

		// fdctRow needs 15-bit fixed-point inputs,
		// but the output of fdctCols would be 12 bits.
		// We do the shift by 3 now instead of in fdctRows,
		// because the multiplies here can take advantage
		// of the extra 3 bits of precision.
		b[0*8+i] = (m5 - 128*8) << 4
		b[4*8+i] = m4 << 4

		// m6, m7 fit in 10 bits (including sign),
		// so we can use tan2_23, which fits in 22 bits (top bit is 0).
		// Shift away 20 bits to leave 3 new bits of precision.
		b[2*8+i] = (tan2_23*m6)>>19 + (m7 << 4)
		b[6*8+i] = (tan2_23*m7)>>19 - (m6 << 4)

		m1, m2 = m1-m2, m1+m2
		// m1, m2 now 10-bit

		m2 = (m2 * invSqrt2_22) >> 17
		m1 = (m1 * invSqrt2_22) >> 17
		// m1, m2 now 15-bit, have extra 2 bits

		m3 <<= 5
		m0 <<= 5
		m3, m1 = m3-m1, m3+m1
		m0, m2 = m0-m2, m0+m2
		// m0, m1, m2, m3 now 16-bit, have extra 2 bits

		m7 = (m3 * tan3_16) >> 16
		m4 = (m0 * tan3_16) >> 16
		// m7 now 16-bit, with extra 2 bits
		// m4 now 16-bit, with extra 2 bits

		m6 = (m1 * tan1_18) >> 18
		m5 = (m2 * tan1_18) >> 18
		// m1 now 16-bit, with extra 2 bits
		// m5 now 16-bit, with extra 2 bits

		b[1*8+i] = (m6 + m2) >> 1
		b[3*8+i] = (m0 - m7) >> 1
		b[5*8+i] = (m3 + m4) >> 1
		b[7*8+i] = (m5 - m1) >> 1
	}
	//fmt.Printf("COLS -> %x\n", *b)
}

func fdctSkalRows(b *block) {
	//fmt.Printf("ROWS %x\n", *b)
	for i := range 8 {
		in := b[i*8 : i*8+8 : i*8+8]
		table := cosine[i]

		// DCT is a unitary operator so we're basically doing the transpose of idctRow.
		a0 := in[0] + in[7]
		b0 := in[0] - in[7]
		a1 := in[1] + in[6]
		b1 := in[1] - in[6]
		a2 := in[2] + in[5]
		b2 := in[2] - in[5]
		a3 := in[3] + in[4]
		b3 := in[3] - in[4]
		// input is 14-bit
		// ax, bx now 15-bit

		// even part
		C2 := int32(table[1])
		C4 := int32(table[3])
		C6 := int32(table[5])
		c0 := a0 + a3
		c1 := a0 - a3
		c2 := a1 + a2
		c3 := a1 - a2
		// cx now 16-bit

		in[0] = (C4 * (c0 + c2)) >> 17
		in[4] = (C4 * (c0 - c2)) >> 17
		in[2] = (C2*c1 + C6*c3) >> 17
		in[6] = (C6*c1 - C2*c3) >> 17

		// odd part
		C1 := int32(table[0])
		C3 := int32(table[2])
		C5 := int32(table[4])
		C7 := int32(table[6])

		in[1] = (C1*b0 + C3*b1 + C5*b2 + C7*b3) >> 17
		in[3] = (C3*b0 - C7*b1 - C1*b2 - C5*b3) >> 17
		in[5] = (C5*b0 - C1*b1 + C7*b2 + C3*b3) >> 17
		in[7] = (C7*b0 - C5*b1 + C3*b2 - C1*b3) >> 17
	}
	//fmt.Printf("ROWS -> %x\n", *b)
}
