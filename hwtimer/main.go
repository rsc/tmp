// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"time"
)

func now() int64

func main() {
	hw := now()
	sw := time.Now()
	for range 10 {
		time.Sleep(100 * time.Millisecond)
		hw1 := now()
		sw1 := time.Now()
		dt := sw1.Sub(sw)
		fmt.Printf("%d in %.1fms (%.1fMHz)\n",
			hw1-hw,
			dt.Seconds()*1e3,
			float64(hw1-hw)/dt.Seconds()/1e6)
		hw = hw1
		sw = sw1
	}
}
