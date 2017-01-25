// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/russross/blackfriday"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		do(os.Stdin)
	} else {
		for _, arg := range args {
			f, err := os.Open(arg)
			if err != nil {
				log.Fatal(err)
			}
			do(f)
			f.Close()
		}
	}
}

var once sync.Once

func do(f *os.File) {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	once.Do(writeHeader)
	os.Stdout.Write(blackfriday.MarkdownCommon(data))
}

func writeHeader() {
	os.Stdout.Write(header)
}

var header = []byte(`<!DOCTYPE html>
<meta charset="UTF-8">
`)
