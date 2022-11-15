// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"log"
	"os"
	"runtime/debug"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("buildinfo: ")

	info, ok := debug.ReadBuildInfo()
	if !ok {
		log.Fatal("no info")
	}

	js, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	js = append(js, '\n')
	os.Stdout.Write(js)
}
