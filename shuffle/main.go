// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Shuffle randomly permutes its input lines.
//
// Usage:
//
//	shuffle [-m max] [file...]
//
// Shuffle reads the named files, or else standard input
// and then prints a random permutation of the input lines.
//
// The -m flag specifies the maximum number of lines to print.
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"
)

var max = flag.Int("m", 0, "maximum number of lines to print")

func main() {
	var all []byte
	flag.Parse()
	if flag.NArg() == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		all = append(all, addNL(data)...)
	} else {
		for _, file := range flag.Args() {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				log.Fatal(err)
			}
			all = append(all, addNL(data)...)
		}
	}

	lines := bytes.SplitAfter(all, []byte{'\n'})
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	n := len(lines)
	if *max > 0 && n > *max {
		n = *max
	}

	rand.Seed(time.Now().UnixNano())
	p := rand.Perm(len(lines))

	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, lines[p[i]]...)
	}
	os.Stdout.Write(out)
}

func addNL(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data
}
