// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"
)

var (
	profile = flag.Bool("profile", false, "record profile")
	n       = flag.Int("n", 20, "number of repetitions")
	zipf    = flag.Bool("zipf", false, "zipf distribution for profile")

	z    = rand.NewZipf(rand.New(rand.NewSource(1)), 2, 10000, 1<<20)
	next int
)

func main() {
	runtime.MemProfileRate = 1
	flag.Parse()
	t0 := time.Now()
	if *profile == true {
		f, err := os.Create("profbench.pprof")
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}

	t1 := time.Now()
	/*
		r := ACMRandom{1}
		for i := 0; i < *n; i++ {
			r0(int(r.Next())>>30, 20)
		}
	*/
	buf := make([]byte, 1024)
	next := int(z.Uint64())
	j := 0
	tj := time.Now()
	var ms1, ms2 runtime.MemStats
	runtime.ReadMemStats(&ms1)
	for i := 0; i < *n; {
		next = r0(next, 20)
		if j++; j == 1e7 {
			now := time.Now()
			runtime.ReadMemStats(&ms2)
			buf = buf[:0]
			buf = append(buf, "BenchmarkRun "...)
			buf = strconv.AppendInt(buf, int64(j), 10)
			buf = append(buf, " "...)
			buf = strconv.AppendFloat(buf, now.Sub(tj).Seconds()*1e9/float64(j), 'f', 3, 64)
			buf = append(buf, " ns/op "...)
			buf = strconv.AppendFloat(buf, float64(ms2.TotalAlloc-ms1.TotalAlloc), 'f', 0, 64)
			buf = append(buf, " B/op "...)
			buf = strconv.AppendFloat(buf, float64(ms2.Mallocs-ms1.Mallocs), 'f', 0, 64)
			buf = append(buf, " allocs/op\n"...)
			os.Stdout.Write(buf)
			runtime.ReadMemStats(&ms1)
			tj = time.Now()
			j = 0
			i++
		}
	}
	t2 := time.Now()

	f, err := os.Create("profbench.mprof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.WriteHeapProfile(f)

	if *profile == true {
		pprof.StopCPUProfile()
	}
	t3 := time.Now()

	fmt.Printf("%.3fs + %.3fs + %.3fs\n", t1.Sub(t0).Seconds(), t2.Sub(t1).Seconds(), t3.Sub(t3).Seconds())

}

func run(n int) {
	for ; n > 0; n-- {
		r0(n, 20)
	}
}

func r0(n int, shift int) int {
	if shift == 0 {
		if *zipf {
			return int(z.Uint64())
		}
		next++
		z.Uint64()
		return next
	}
	if (n>>uint(shift))&1 != 0 {
		return r1(n, shift-1)
	} else {
		return r0(n, shift-1)
	}
}

func r1(n int, shift int) int {
	if shift == 0 {
		return int(z.Uint64())
	}
	if (n>>uint(shift))&1 != 0 {
		return r1(n, shift-1)
	} else {
		return r0(n, shift-1)
	}
}

type ACMRandom struct{ seed int32 }

func (r *ACMRandom) Next() int32 {
	const (
		M = 2147483647
		A = 16807
	)
	// seed = (seed * A) % M, where M = 2³¹-1
	lo := uint32(A * (r.seed & 0xFFFF))
	hi := uint32(A * int32(uint32(r.seed)>>16))
	lo += (hi & 0x7FFF) << 16
	if lo > M {
		lo &= M
		lo++
	}
	lo += hi >> 15
	if lo > M {
		lo &= M
		lo++
	}
	r.seed = int32(lo)
	return int32(lo)
}
