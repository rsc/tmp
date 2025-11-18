// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Translated from DoubleToDecimal.java.

/*
 * Copyright 2018-2020 Raffaello Giulietti
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

/*
   For full details about this code see the following references:

   [1] Giulietti, "The Schubfach way to render doubles",
       https://drive.google.com/open?id=1luHhyQF9zKlM8yJ1nebU0OgVYhfC6CBN

   [2] IEEE Computer Society, "IEEE Standard for Floating-Point Arithmetic"

   [3] Bouvier & Zimmermann, "Division-Free Binary-to-Decimal Conversion"

   Divisions are avoided altogether for the benefit of those architectures
   that do not provide specific machine instructions or where they are slow.
   This is discussed in section 10 of [1].
*/

package schubfach

import (
	"math"
	"math/bits"
)

const (
	// The precision in bits.
	_P = 53

	// Exponent width in bits.
	_W = (64 - 1) - (_P - 1)

	// Minimum value of the exponent: -(2^(W-1)) - P + 3.
	_Q_MIN = (-1<<_W - 1) - _P + 3

	// Maximum value of the exponent: 2^(W-1) - P.
	_Q_MAX = (1<<_W - 1) - _P

	// 10^(E_MIN - 1) <= MIN_VALUE < 10^E_MIN
	_E_MIN = -323

	// 10^(E_MAX - 1) <= MAX_VALUE < 10^E_MAX
	_E_MAX = 309

	// Threshold to detect tiny values, as in section 8.1.1 of [1]
	_C_TINY = 3

	// The minimum and maximum k, as in section 8 of [1]
	_K_MIN = -324
	_K_MAX = 292

	// H is as in section 8 of [1].
	_H = 17

	// Minimum value of the significand of a normal value: 2^(P-1).
	_C_MIN = 1<<_P - 1

	// Mask to extract the biased exponent.
	_BQ_MASK = (1 << _W) - 1

	// Mask to extract the fraction bits.
	_T_MASK = (1<<_P - 1) - 1

	// Used in rop().
	_MASK_63 = (1 << 63) - 1

	// Used for left-to-tight digit extraction.
	_MASK_28 = (1 << 28) - 1
)

func Ftoa(v float64) (digits uint64, exp int) {
	/*
	   For full details see references [2] and [1].

	   For finite v != 0, determine integers c and q such that
	       |v| = c 2^q    and
	       Q_MIN <= q <= Q_MAX    and
	           either    2^(P-1) <= c < 2^P                 (normal)
	           or        0 < c < 2^(P-1)  and  q = Q_MIN    (subnormal)
	*/
	bits := math.Float64bits(v)
	t := bits & _T_MASK
	bq := int(bits>>(_P-1)) & _BQ_MASK
	if bq == _BQ_MASK {
		panic("non-number")
	}
	if bq != 0 {
		// normal value. Here mq = -q
		mq := -_Q_MIN + 1 - bq
		c := _C_MIN | t
		// The fast path discussed in section 8.2 of [1].
		if 0 < mq && mq < _P {
			f := c >> mq
			if f<<mq == c {
				return f, 0
			}
		}
		return toDecimal(-mq, c, 0)
	}

	// subnormal value
	if t < _C_TINY {
		return toDecimal(_Q_MIN, 10*t, -1)
	}
	return toDecimal(_Q_MIN, t, 0)
}

func toDecimal(q int, c uint64, dk int) (digits uint64, exp int) {
	/*
	   The skeleton corresponds to figure 4 of [1].
	   The efficient computations are those summarized in figure 7.

	   Here's a correspondence between Java names and names in [1],
	   expressed as approximate LaTeX source code and informally.
	   Other names are identical.
	   cb:     \bar{c}     "c-bar"
	   cbr:    \bar{c}_r   "c-bar-r"
	   cbl:    \bar{c}_l   "c-bar-l"

	   vb:     \bar{v}     "v-bar"
	   vbr:    \bar{v}_r   "v-bar-r"
	   vbl:    \bar{v}_l   "v-bar-l"

	   rop:    r_o'        "r-o-prime"
	*/

	out := c & 1
	cb := c << 2
	cbr := cb + 2
	/*
	   flog10pow2(e) = floor(log_10(2^e))
	   flog10threeQuartersPow2(e) = floor(log_10(3/4 2^e))
	   flog2pow10(e) = floor(log_2(10^e))
	*/
	var cbl uint64
	var k int
	if c != _C_MIN || q == _Q_MIN {
		// regular spacing
		cbl = cb - 2
		k = flog10pow2(q)
	} else {
		// irregular spacing
		cbl = cb - 1
		k = flog10threeQuartersPow2(q)
	}
	h := q + flog2pow10(-k) + 2

	// g1 and g0 are as in section 9.9.3 of [1], so g = g1 2^63 + g0
	g1 := g1(k)
	g0 := g0(k)

	vb := rop(g1, g0, cb<<h)
	vbl := rop(g1, g0, cbl<<h)
	vbr := rop(g1, g0, cbr<<h)

	s := vb >> 2
	if s >= 100 {
		/*
		   For n = 17, m = 1 the table in section 10 of [1] shows
		       s' = floor(s / 10) = floor(s 115_292_150_460_684_698 / 2^60)
		          = floor(s 115_292_150_460_684_698 2^4 / 2^64)

		   sp10 = 10 s'
		   tp10 = 10 t'
		   upin    iff    u' = sp10 10^k in Rv
		   wpin    iff    w' = tp10 10^k in Rv
		   See section 9.4 of [1].
		*/
		h, _ := bits.Mul64(s, 115_292_150_460_684_698<<4)
		sp10 := 10 * h
		tp10 := sp10 + 10
		upin := vbl+out <= sp10<<2
		wpin := (tp10<<2)+out <= vbr
		if upin != wpin {
			if upin {
				return sp10, k
			}
			return tp10, k
		}
	}

	/*
	   10 <= s < 100    or    s >= 100  and  u', w' not in Rv
	   uin    iff    u = s 10^k in Rv
	   win    iff    w = t 10^k in Rv
	   See section 9.4 of [1].
	*/
	t := s + 1
	uin := vbl+out <= s<<2
	win := (t<<2)+out <= vbr
	if uin != win {
		// Exactly one of u or w lies in Rv.
		if uin {
			return s, k + dk
		}
		return t, k + dk
	}
	/*
	   Both u and w lie in Rv: determine the one closest to v.
	   See section 9.4 of [1].
	*/
	cmp := vb - (s + t<<1)
	if cmp < 0 || cmp == 0 && s&1 == 0 {
		return s, k + dk
	}
	return t, k + dk
}

/*
Computes rop(cp g 2^(-127)), where g = g1 2^63 + g0
See section 9.10 and figure 5 of [1].
*/
func rop(g1, g0, cp uint64) uint64 {
	x1, _ := bits.Mul64(g0, cp)
	y0 := g1 * cp
	y1, _ := bits.Mul64(g1, cp)
	z := (y0 >> 1) + x1
	vbp := y1 + (z >> 63)
	return vbp | ((z&_MASK_63)+_MASK_63)>>63
}

func y(a int) int {
	/*
	   Algorithm 1 in [3] needs computation of
	       floor((a + 1) 2^n / b^k) - 1
	   with a < 10^8, b = 10, k = 8, n = 28.
	   Noting that
	       (a + 1) 2^n <= 10^8 2^28 < 10^17
	   For n = 17, m = 8 the table in section 10 of [1] leads to:
	*/
	h, _ := bits.Mul64((uint64(a)+1)<<28, 193_428_131_138_340_668)
	return int(h>>20 - 1)
}
