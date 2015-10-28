// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand"
	"testing"
)

func benchmarkRand(b *testing.B, n int) {
	c := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() {
			sum := 0
			for j := 0; j < b.N/n; j++ {
				sum += rand.Int()
			}
			c <- sum
		}()
	}
	for i := 0; i < n; i++ {
		<-c
	}
}

func BenchmarkRand1(b *testing.B)   { benchmarkRand(b, 1) }
func BenchmarkRand2(b *testing.B)   { benchmarkRand(b, 2) }
func BenchmarkRand4(b *testing.B)   { benchmarkRand(b, 4) }
func BenchmarkRand8(b *testing.B)   { benchmarkRand(b, 8) }
func BenchmarkRand12(b *testing.B)  { benchmarkRand(b, 12) }
func BenchmarkRand16(b *testing.B)  { benchmarkRand(b, 16) }
func BenchmarkRand32(b *testing.B)  { benchmarkRand(b, 32) }
func BenchmarkRand64(b *testing.B)  { benchmarkRand(b, 64) }
func BenchmarkRand128(b *testing.B) { benchmarkRand(b, 128) }
func BenchmarkRand256(b *testing.B) { benchmarkRand(b, 256) }
