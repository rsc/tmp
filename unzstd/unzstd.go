// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"log"
	"os"

	"github.com/klauspost/compress/zstd"
)

func main() {
	dec, err := zstd.NewReader(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(os.Stdout, dec)
	if err != nil {
		log.Fatal(err)
	}
}
