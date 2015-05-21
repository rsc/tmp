// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chain_test

import "testing"

const goroutines = 100

var (
	cin  chan<- int
	cout <-chan int

	cinbuf  chan<- int
	coutbuf <-chan int
)

func init() {
	c := make(chan int)
	cin = c
	for i := 0; i < goroutines; i++ {
		next := make(chan int)
		go copy1(c, next)
		c = next
	}
	cout = c

	c = make(chan int, 1)
	cinbuf = c
	for i := 0; i < goroutines; i++ {
		next := make(chan int, 1)
		go copy1(c, next)
		c = next
	}
	coutbuf = c

}

func copy1(from, to chan int) {
	for {
		to <- <-from
	}
}

func BenchmarkChain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cin <- 1
		<-cout
	}
}

func BenchmarkChainBuf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cinbuf <- 1
		<-coutbuf
	}
}
