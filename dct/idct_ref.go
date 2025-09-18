// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dct

func idctFloat(b *block) {
	var dst [8 * 8]float64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			sum := 0.0
			for v := 0; v < 8; v++ {
				for u := 0; u < 8; u++ {
					sum += alpha(u) * alpha(v) * float64(b[8*v+u]) *
						cosFloat[((2*x+1)*u)%32] *
						cosFloat[((2*y+1)*v)%32]
				}
			}
			dst[8*y+x] = sum / 8
		}
	}
	// Convert from float64 to int32.
	for i := range dst {
		b[i] = int32(dst[i] + 0.5)
	}
}
