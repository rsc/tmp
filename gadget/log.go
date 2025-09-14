// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"math/rand/v2"
	"path/filepath"
	"math/big"
	"time"
	"encoding/json"
	"log"
	"os"
)

func logFile() *os.File {
	dir := filepath.Join(home, ".gadget/log")
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0700); err != nil {
			log.Fatal(err)
		}
	}
	file := time.Now().UTC().Format("2006-01-02-150405")
	id := big.NewInt(int64(rand.Int64())).Text(36)
	for len(id) < 10 {
		id = "0" + id
	}
	file += "-" + id[:7]

	f, err := os.Create(filepath.Join(dir, file))
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func logJSON(f *os.File, verb string, arg any) {
	line := []byte(verb)
	if arg != nil {
		js, err := json.Marshal(arg)
		if err != nil {
			log.Fatal(err)
		}
		line = append(line, ' ')
		line = append(line, js...)
	}
	line = append(line, '\n')
	if _, err := f.Write(line); err != nil {
		log.Fatal(err)
	}
}
