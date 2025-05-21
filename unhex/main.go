// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Unhex is the opposite of hexdump -C or Plan 9's "xd -b".
package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

// parseHexdump parses the hex dump in text, which should be the
// output of "hexdump -C" or Plan 9's "xd -b",
// and returns the original data used to produce the dump.
// It is meant to enable storing golden binary files as text, so that
// changes to the golden files can be seen during code reviews.
func parseHexdump(text string) ([]byte, error) {
	var out []byte
	for _, line := range strings.Split(text, "\n") {
		if i := strings.Index(line, "|"); i >= 0 { // remove text dump
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) > 1+16 {
			return nil, fmt.Errorf("parsing hex dump: too many fields on line %q", line)
		}
		if len(f) == 0 || len(f) == 1 && f[0] == "*" { // all zeros block omitted
			continue
		}
		addr64, err := strconv.ParseUint(f[0], 16, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing hex dump: invalid address %q", f[0])
		}
		addr := int(addr64)
		if len(out) < addr {
			out = append(out, make([]byte, addr-len(out))...)
		}
		for _, x := range f[1:] {
			val, err := strconv.ParseUint(x, 16, 8)
			if err != nil {
				return nil, fmt.Errorf("parsing hexdump: invalid hex byte %q", x)
			}
			out = append(out, byte(val))
		}
	}
	return out, nil
}

func main() {
	hex, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	data, err := parseHexdump(string(hex))
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.Write(data)
}
