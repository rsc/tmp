// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ftoa implements a fixed-width floating point formatting
// algorithm and benchmarks to compare with other algorithms.
package ftoa

import (
	"math"
	"math/bits"
)

// TODO add test that breaks with prec=18

func Ftoa(f float64, prec int) (dm uint64, dp int) {
	return ftoa(f, prec)
}

// ftoa(f, prec) returns the prec-digit decimal form of f as dm * 10^dp.
func ftoa(f float64, prec int) (dm uint64, dp int) {
	if prec > 18 {
		panic("ftoa called with prec > 18")
	}

	// Split floating-point number into parts.
	fr, fe := math.Frexp(f)
	fm := uint64(fr * (1 << 64))
	fe -= 64

	// Count how many digits p we need to scale by.
	p := prec - 1 - mulLog10_2(63+fe)

	// Multiply by 10**p.
	pm, pe := pow10(p)
	de := fe + pe
	s := -de - 1
	dh, _ := bits.Mul64(fm, pm.hi)
	dt := uint64(1)
	switch {
	case 0 <= p && p <= 27:
		dt = bool2int(dh&(1<<s-1) != 0 || fm*pm.hi != 0)
	case -22 <= p && p <= -1 && divisiblePow5(fm, -p):
		dh++
		dt = bool2int(dh&(1<<s-1) != 0)
	case dh&(1<<s-1) == 1<<s-1:
		dl, _ := bits.Mul64(fm, pm.lo)
		_, carry := bits.Add64(dl, fm*pm.hi, 0)
		dh += carry
	}
	dmr := dh >> s

	// Remove potential extra digit.
	dp = -p
	max := uint64pow10[prec]
	if dmr>>1 >= max {
		dt |= bool2int(dmr%10 != 0)
		dmr /= 10
		dp++
	}

	// Round the result to an integer dm.
	dm = (dmr + 1&(dt|dmr>>1)) >> 1
	if dm == max {
		dm = uint64pow10[prec-1]
		dp++
	}

	// Report the answer as dm * 10^dp.
	return dm, dp
}

func efmt(dst []byte, f float64, prec int) []byte {
	dm, dp := ftoa(f, prec)
	for i := prec; i >= 1; i-- {
		dst[i] = byte('0' + dm%10)
		dm /= 10
	}
	dp += prec - 1
	dst[0] = dst[1]
	dst[1] = '.'
	dst[prec+1] = 'e'
	if dp < 0 {
		dst[prec+2] = '-'
		dp = -dp
	} else {
		dst[prec+2] = '+'
	}
	if dp < 10 {
		dst[prec+3] = byte('0' + dp)
		return dst[:prec+4]
	}
	if dp < 100 {
		dst[prec+3] = byte('0' + dp/10)
		dst[prec+4] = byte('0' + dp%10)
		return dst[:prec+5]
	}
	dst[prec+3] = byte('0' + dp/100)
	dst[prec+4] = byte('0' + dp/10%10)
	dst[prec+5] = byte('0' + dp%10)
	return dst[:prec+6]
}

// divisiblePow5 reports whether x is divisible by 5^p.
// It returns false for p not in [1, 22],
// because we only care about float64 mantissas, and 5^23 > 2^53.
func divisiblePow5(x uint64, p int) bool {
	return 1 <= p && p <= 22 && x*div5Tab[p-1][0] <= div5Tab[p-1][1]
}

// maxUint64 is the largest possible uint64.
const maxUint64 = 1<<64 - 1

// div5Tab[p-1] is the multiplicative inverse of 5**p and maxUint64/5**p.
var div5Tab = [22][2]uint64{
	{0xcccccccccccccccd, maxUint64 / 5},
	{0x8f5c28f5c28f5c29, maxUint64 / 25},
	{0x1cac083126e978d5, maxUint64 / 125},
	{0xd288ce703afb7e91, maxUint64 / 625},
	{0x5d4e8fb00bcbe61d, maxUint64 / 3125},
	{0x790fb65668c26139, maxUint64 / 15625},
	{0xe5032477ae8d46a5, maxUint64 / 78125},
	{0xc767074b22e90e21, maxUint64 / 390625},
	{0x8e47ce423a2e9c6d, maxUint64 / 1953125},
	{0x4fa7f60d3ed61f49, maxUint64 / 9765625},
	{0x0fee64690c913975, maxUint64 / 48828125},
	{0x3662e0e1cf503eb1, maxUint64 / 244140625},
	{0xa47a2cf9f6433fbd, maxUint64 / 1220703125},
	{0x54186f653140a659, maxUint64 / 6103515625},
	{0x7738164770402145, maxUint64 / 30517578125},
	{0xe4a4d1417cd9a041, maxUint64 / 152587890625},
	{0xc75429d9e5c5200d, maxUint64 / 762939453125},
	{0xc1773b91fac10669, maxUint64 / 3814697265625},
	{0x26b172506559ce15, maxUint64 / 19073486328125},
	{0xd489e3a9addec2d1, maxUint64 / 95367431640625},
	{0x90e860bb892c8d5d, maxUint64 / 476837158203125},
	{0x502e79bf1b6f4f79, maxUint64 / 2384185791015625},
}

func bool2int(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

// mulLog10_2(x) returns ⌊x * log_10 2⌋
func mulLog10_2(x int) int {
	// log(2)/log(10) ≈ 0.30102999566 ≈ 78913 / 2^18
	return (x * 78913) >> 18
}

var uint64pow10 = [...]uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9,
	1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

// A uint128 is a 128-bit uint.
type uint128 struct {
	hi uint64
	lo uint64
}

// pow10 returns the 128-bit mantissa and binary exponent of 10**e.
// That is, 10^e = mant/2^128 * 2**exp.
// If e is out of range, pow10 returns ok=false.
func pow10(e int) (mant uint128, exp int) {
	if e < pow10Min || e > pow10Max {
		panic("pow10")
	}
	return pow10Tab[e-pow10Min], 1 + mulLog2_10(e)
}

// mulLog2_10(x) returns ⌊x * log_10 2⌋
func mulLog2_10(x int) int {
	// log(10)/log(2) ≈ 3.32192809489 ≈ 108853 / 2^15
	return (x * 108853) >> 15
}
