// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package go124

import "math"

func ryu(tmp *[32]byte, f float64, prec int) int {
	flt := &float64info
	bits := math.Float64bits(f)
	exp := int(bits>>flt.mantbits) & (1<<flt.expbits - 1)
	mant := bits & (uint64(1)<<flt.mantbits - 1)

	switch exp {
	case 0:
		exp++
	default:
		mant |= uint64(1) << flt.mantbits
	}
	exp += flt.bias

	var d decimalSlice
	var buf [32]byte
	d.d = buf[:]
	ryuFtoaFixed64(&d, mant, exp-int(flt.mantbits), prec)
	return len(formatDigits(tmp[:0], false, false, d, prec-1, 'e'))
}

func LoopRyu(dst []byte, n int, f float64, prec int) []byte {
	var tmp [32]byte
	i := 0
	for range n {
		i = ryu(&tmp, f, prec)
	}
	return append(dst, tmp[:i]...)
}

func LoopSumRyu(n int, fs []float64, prec int) int64 {
	var buf [32]byte
	var out int64
	for range n {
		total := int64(0)
		for _, f := range fs {
			ryu(&buf, f, prec)
			total += int64(buf[0])
		}
		out = total
	}
	return out
}

func unopt(tmp *[32]byte, f float64, prec int) int {
	flt := &float64info
	bits := math.Float64bits(f)
	exp := int(bits>>flt.mantbits) & (1<<flt.expbits - 1)
	mant := bits & (uint64(1)<<flt.mantbits - 1)

	switch exp {
	case 0:
		exp++
	default:
		mant |= uint64(1) << flt.mantbits
	}
	exp += flt.bias

	return len(bigFtoa(tmp[:0], prec-1, 'e', false, mant, exp, flt))
}

func LoopUnopt(dst []byte, n int, f float64, prec int) []byte {
	var tmp [32]byte
	i := 0
	for range n {
		i = unopt(&tmp, f, prec)
	}
	return append(dst, tmp[:i]...)
}

func LoopSumUnopt(n int, fs []float64, prec int) int64 {
	var buf [32]byte
	var out int64
	for range n {
		total := int64(0)
		for _, f := range fs {
			unopt(&buf, f, prec)
			total += int64(buf[0])
		}
		out = total
	}
	return out
}
