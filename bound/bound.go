// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bound is a simple benchmark of the x86 BOUND instruction.
// Spoiler alert. It's slow.
//
// Results
//
// On my MacBook Pro (Intel Core i5-3210M @ 2.50 GHz),
// using BOUND is 9x slower than using CMP
// after you subtract out the cost of the benchmark loop:
//
//	g% 386 go test -bench . -benchtime 10s
//	testing: warning: no tests to run
//	PASS
//	BenchmarkNop	1000000000	         0.33 ns/op
//	BenchmarkBound	1000000000	         3.35 ns/op
//	BenchmarkCmp	1000000000	         0.66 ns/op
//	BenchmarkCmpReg	1000000000	         0.67 ns/op
//	ok  	rsc.io/tmp/bound	9.147s
//	g%
//
// Maybe someone familiar with the CPU architecture of the Core i5
// can explain to me why I have a 2.50 GHz clock speed but
// instruction retirement seems to be running at 3 GHz.
//
package bound
