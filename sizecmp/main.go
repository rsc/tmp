// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: sizecmp binary1 binary2\n")
	os.Exit(2)
}

func main() {
	if len(os.Args) != 3 {
		usage()
	}

	size1 := readSize(os.Args[1])
	size2 := readSize(os.Args[2])

	var keys []string
	for k := range size1 {
		keys = append(keys, k)
	}
	for k := range size2 {
		if _, ok := size1[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var total1, total2 int64
	for _, k := range keys {
		fmt.Printf("%-30s %11d %11d %+11d\n", k, size1[k], size2[k], size2[k]-size1[k])
		total1 += size1[k]
		total2 += size2[k]
	}
	fmt.Printf("%30s %11d %11d %+11d\n", "total", total1, total2, total2-total1)
}

func readSize(file string) map[string]int64 {
	out, err := exec.Command("otool", "-l", file).CombinedOutput()
	if err != nil {
		log.Fatalf("otool -l %s: %v\n%s", file, err, out)
	}
	var name string
	sizes := make(map[string]int64)
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		if f[0] == "sectname" {
			name = f[1]
		}
		if f[0] == "size" {
			n, err := strconv.ParseInt(f[1], 0, 64)
			if err == nil {
				sizes[name] += n
			}
		}
	}
	return sizes
}
