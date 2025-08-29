// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/licensecheck"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: lcheck [-t threshold] file...\n")
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	for _, arg := range flag.Args() {
		data, err := ioutil.ReadFile(arg)
		if err != nil {
			log.Fatal(err)
		}

		cov := licensecheck.Scan(data)
		fmt.Printf("%s: %.1f%%\n", arg, cov.Percent)
		last := 0
		for _, m := range cov.Match {
			if m.Start-last > 2000 {
				fmt.Printf("\tGAP :#%d,#%d (%d)\n", last, m.Start, m.Start-last)
			}
			fmt.Printf("\t%s :#%d,#%d\n", m.ID, m.Start, m.End)
			last = m.End
		}
	}
}
