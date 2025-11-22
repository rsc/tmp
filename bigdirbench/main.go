// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var dir = flag.String("d", "/tmp", "path in which to create test directory")
var n = flag.Int("n", 1000000, "number of files to create")

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	d, err := os.MkdirTemp(*dir, "bigdirbench-")
	check(err)
	fmt.Printf("working in %s\n", d)
	wd, err := os.Getwd()
	check(err)
	check(os.Chdir(d))

	for i := 0; i < *n; {
		end := i + i/10
		pow := 10
		for pow*100 < end {
			pow *= 10
		}
		end = end / pow * pow
		if end > *n {
			end = *n + 1
		}
		if end <= i {
			end = i + 1
		}
		var name string
		for ; i < end; i++ {
			name = fmt.Sprintf("%032d", i)
			f, err := os.Create(name)
			check(err)
			f.Close()
			i++
		}
		t := time.Now()
		_, err := os.Stat(name)
		check(err)
		dt := time.Since(t)
		t = time.Now()
		f, err := os.Open(".")
		check(err)
		_, err = f.Readdirnames(0)
		check(err)
		f.Close()
		dt2 := time.Since(t)
		fmt.Printf("%d %.6f %.6f\n", i, dt.Seconds(), dt2.Seconds())
	}

	check(os.Chdir(wd))
	check(os.RemoveAll(d))
}
