// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This simple program benchmarks the BOUND instruction
// against an explicit CMP and conditional jump.
// It must be compiled for 386, since BOUND is not available
// on amd64.

package bound

import "testing"

func bound(int32)
func cmp(int32)
func cmpreg(int32)
func nop(int32)

func bench(b *testing.B, f func(int32)) {
	x := int(b.N / 1e9)
	for i := 0; i < x; i++ {
		f(1e9)
	}
	f(int32(b.N % 1e9))
}

func BenchmarkNop(b *testing.B) { bench(b, nop) }
func BenchmarkBound(b *testing.B) { bench(b, bound) }
func BenchmarkCmp(b *testing.B) { bench(b, cmp) }
func BenchmarkCmpReg(b *testing.B) { bench(b, cmpreg) }
