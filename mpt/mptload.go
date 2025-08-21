// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Mptload loads a list of keys into a database.
package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"log"
	"os"

	"rsc.io/tmp/mpt"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: mptload db1 db2 keys.txt\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("mptload: ")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 3 {
		usage()
	}

	file1, file2, keys := flag.Arg(0), flag.Arg(1), flag.Arg(2)
	tree, err := mpt.Create(file1, file2)
	if err != nil {
		log.Fatal(err)
	}

	data, err := os.ReadFile(keys)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for line := range bytes.Lines(data) {
		h := sha256.Sum256(line)
		if err := tree.Set(h, h); err != nil {
			log.Fatal(err)
		}
		n++
		if n%1000000 == 0 {
			log.Printf("stored %d", n)
		}
	}
	log.Print("snap")
	if _, err := tree.Snap(); err != nil {
		log.Fatal(err)
	}
	log.Print("sync")
	if err := tree.Sync(); err != nil {
		log.Fatal(err)
	}
	log.Print("done")
}
