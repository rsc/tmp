// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	p          = flag.Int("p", 4, "number of workers")
	cpuprofile = flag.String("cpuprofile", "", "write CPU profile to `file`")
	tracefile  = flag.String("trace", "", "write trace to `file`")
	lock       = flag.String("lock", "nop", "locking type")
)

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
	}
	if *tracefile != "" {
		f, err := os.Create(*tracefile)
		if err != nil {
			log.Fatal(err)
		}
		trace.Start(f)
	}

	times = make([][]int, *p)
	for i := range times {
		times[i] = make([]int, 0, 1000)
	}

	l1, l2, l3, l4 = newLock[*lock](), newLock[*lock](), newLock[*lock](), newLock[*lock]()

	t := time.Now()
	burnCPU1()
	fmt.Printf("burn1: %v\n", time.Since(t))

	var ru, ru2 syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru2)
	start := time.Now()

	req := make(chan bool)
	go sendRequests(req)
	var wg sync.WaitGroup
	for i := 0; i < *p; i++ {
		wg.Add(1)
		go worker(req, i, &wg)
	}
	time.Sleep(10 * time.Second)
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru2)
	elapsed := time.Since(start)
	atomic.StoreUint32(&done, 1)

	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
	if *tracefile != "" {
		trace.Stop()
	}
	wg.Wait()

	fmt.Printf("%v elapsed, %v user, %v system\n", elapsed, time.Duration(syscall.TimevalToNsec(ru2.Utime)-syscall.TimevalToNsec(ru.Utime)), time.Duration(syscall.TimevalToNsec(ru2.Stime)-syscall.TimevalToNsec(ru.Stime)))

	fmt.Printf("workers:\n")
	for i := 0; i < *p; i++ {
		fmt.Printf("%v\n", times[i])
	}
}

var done uint32
var l1, l2, l3, l4 sync.Locker
var times [][]int

var newLock = map[string]func() sync.Locker{
	"nop":   func() sync.Locker { return NopLock{} },
	"mutex": func() sync.Locker { return new(sync.Mutex) },
	"chan":  func() sync.Locker { return NewChanLock() },
}

type ChanLock chan bool

func (c ChanLock) Lock()   { <-c }
func (c ChanLock) Unlock() { c <- true }

func NewChanLock() sync.Locker {
	c := make(ChanLock, 1)
	c <- true
	return c
}

type NopLock struct{}

func (NopLock) Lock()   {}
func (NopLock) Unlock() {}

func sendRequests(req chan bool) {
	for atomic.LoadUint32(&done) == 0 {
		req <- true
	}
	close(req)
}

func worker(req chan bool, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	ts := times[i]
	for range req {
		l1.Lock()
		t := time.Now()
		burnCPU1()
		ts = append(ts, int(time.Since(t)/time.Millisecond))
		l1.Unlock()
		l2.Lock()
		t = time.Now()
		burnCPU2()
		ts = append(ts, int(time.Since(t)/time.Millisecond))
		l2.Unlock()
		l3.Lock()
		t = time.Now()
		burnCPU3()
		ts = append(ts, int(time.Since(t)/time.Millisecond))
		l3.Unlock()
		l4.Lock()
		t = time.Now()
		burnCPU4()
		ts = append(ts, int(time.Since(t)/time.Millisecond))
		l4.Unlock()
	}
	times[i] = ts
}

const (
	Burn = 1300
	nmax = 5552
	mod  = 65521
)

func burnCPU1() {
	// adler32 repeated on 64-byte buffer
	var b [64]byte
	n := Burn
	for ; n >= 0; n-- {
		for j := 0; j < 500; j++ {
			d := uint32(0)
			p := b[:]
			s1, s2 := uint32(d&0xffff), uint32(d>>16)
			for len(p) > 0 {
				var q []byte
				if len(p) > nmax {
					p, q = p[:nmax], p[nmax:]
				}
				for len(p) >= 4 {
					s1 += uint32(p[0])
					s2 += s1
					s1 += uint32(p[1])
					s2 += s1
					s1 += uint32(p[2])
					s2 += s1
					s1 += uint32(p[3])
					s2 += s1
					p = p[4:]
				}
				for _, x := range p {
					s1 += uint32(x)
					s2 += s1
				}
				s1 %= mod
				s2 %= mod
				p = q
			}
			d = s2<<16 | s1
			b[0] += byte(d)
		}
	}
}

func burnCPU2() {
	// adler32 repeated on 64-byte buffer
	var b [64]byte
	n := Burn
	for ; n >= 0; n-- {
		for j := 0; j < 500; j++ {
			d := uint32(0)
			p := b[:]
			s1, s2 := uint32(d&0xffff), uint32(d>>16)
			for len(p) > 0 {
				var q []byte
				if len(p) > nmax {
					p, q = p[:nmax], p[nmax:]
				}
				for len(p) >= 4 {
					s1 += uint32(p[0])
					s2 += s1
					s1 += uint32(p[1])
					s2 += s1
					s1 += uint32(p[2])
					s2 += s1
					s1 += uint32(p[3])
					s2 += s1
					p = p[4:]
				}
				for _, x := range p {
					s1 += uint32(x)
					s2 += s1
				}
				s1 %= mod
				s2 %= mod
				p = q
			}
			d = s2<<16 | s1
			b[0] += byte(d)
		}
	}
}

func burnCPU3() {
	// adler32 repeated on 64-byte buffer
	var b [64]byte
	n := Burn
	for ; n >= 0; n-- {
		for j := 0; j < 500; j++ {
			d := uint32(0)
			p := b[:]
			s1, s2 := uint32(d&0xffff), uint32(d>>16)
			for len(p) > 0 {
				var q []byte
				if len(p) > nmax {
					p, q = p[:nmax], p[nmax:]
				}
				for len(p) >= 4 {
					s1 += uint32(p[0])
					s2 += s1
					s1 += uint32(p[1])
					s2 += s1
					s1 += uint32(p[2])
					s2 += s1
					s1 += uint32(p[3])
					s2 += s1
					p = p[4:]
				}
				for _, x := range p {
					s1 += uint32(x)
					s2 += s1
				}
				s1 %= mod
				s2 %= mod
				p = q
			}
			d = s2<<16 | s1
			b[0] += byte(d)
		}
	}
}

func burnCPU4() {
	// adler32 repeated on 64-byte buffer
	var b [64]byte
	n := Burn
	for ; n >= 0; n-- {
		for j := 0; j < 500; j++ {
			d := uint32(0)
			p := b[:]
			s1, s2 := uint32(d&0xffff), uint32(d>>16)
			for len(p) > 0 {
				var q []byte
				if len(p) > nmax {
					p, q = p[:nmax], p[nmax:]
				}
				for len(p) >= 4 {
					s1 += uint32(p[0])
					s2 += s1
					s1 += uint32(p[1])
					s2 += s1
					s1 += uint32(p[2])
					s2 += s1
					s1 += uint32(p[3])
					s2 += s1
					p = p[4:]
				}
				for _, x := range p {
					s1 += uint32(x)
					s2 += s1
				}
				s1 %= mod
				s2 %= mod
				p = q
			}
			d = s2<<16 | s1
			b[0] += byte(d)
		}
	}
}
