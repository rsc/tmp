// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"os"

	"rsc.io/tmp/brotli"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "-u" {
		r := brotli.NewReader(os.Stdin)
		_, err := io.Copy(os.Stdout, r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "brotli: %v\n", err)
			os.Exit(2)
		}
		return
	}

	w := brotli.NewWriter(os.Stdout, brotli.WriterOptions{LGWin: 24, Quality: 11})
	_, err := io.Copy(w, os.Stdin)
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "brotli: %v\n", err)
		os.Exit(2)
	}
}
