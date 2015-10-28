// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"sync"
	"time"
)

var (
	doublelock = flag.Bool("2", false, "use two locks")
	duration   = flag.Duration("t", 100*time.Microsecond, "time to sleep")
)

func main() {
	flag.Parse()

	done := make(chan bool)
	var mu sync.Mutex
	var mu2 sync.Mutex
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if *doublelock {
					mu2.Lock()
				}
				mu.Lock()
				if *doublelock {
					mu2.Unlock()
				}
				time.Sleep(*duration)
				mu.Unlock()
			}
		}
	}()

	lastPrint := time.Now()
	var sum time.Duration
	for i := 0; ; i++ {
		time.Sleep(*duration)
		start := time.Now()
		if *doublelock {
			mu2.Lock()
		}
		mu.Lock()
		if *doublelock {
			mu2.Unlock()
		}
		now := time.Now()
		mu.Unlock()
		elapsed := now.Sub(start)
		sum += elapsed
		if i == 0 || now.Sub(lastPrint) > 1*time.Second {
			log.Printf("lock#%d took %.6fs; average %.6fs\n", i, elapsed.Seconds(), (sum / time.Duration(i+1)).Seconds())
			lastPrint = now
		}
	}
}
