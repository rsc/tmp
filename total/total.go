// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Total totals columns of its input.
//
// Usage:
//
//	total [-F regexp | -csv] [file...]
//
// Total sums the values in the named files (or else standard input).
// Each file is read as a sequence of records, one per line.
// Each line is read as a list of space-separated fields
// in the manner of Go's [strings.Fields].
// Total prints the sum of all the corresponding fields in the records:
// the total of all the first fields, the total of all the second fields, and so on.
//
// If a given field in any record is non-numeric, meaning it cannot be
// parsed by Go's [strconv.ParseFloat], then the sum for that field is printed as "~".
// As a special case, if all records contain the same non-numeric string
// for a given field, then the sum for that field is that string.
//
// The -F flag specifies a Go regular expression to use as an input field
// separator instead of using [strings.Fields]. It does not affect the output,
// which is printed with single spaces between all fields.
//
// The -csv flag specifies that the input lines should be treated as CSV data
// and that the output should also be printed as CSV data.
//
// # Example
//
// Totaling the output of “ls -l” shows that the sources for this program
// currently total 3,892 bytes:
//
//	% ls -l
//	total 16
//	-rw-r--r--  1 rsc  primarygroup    35 Oct 29 08:20 go.mod
//	-rw-r--r--  1 rsc  primarygroup  3857 Oct 29 08:48 total.go
//	% ls -l | total
//	~ 18 rsc primarygroup 3892 Oct 58 ~ ~
//	%
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	Fflag   = flag.String("F", "", "use `regexp` to match field separators")
	csvflag = flag.Bool("csv", false, "treat input and output as CSV")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: total [-F regexp | -csv] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var split = strings.Fields

type out struct {
	f   float64
	i   int64
	s   string
	fok bool
	iok bool
}

var total []out

func main() {
	log.SetPrefix("total: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if *Fflag != "" {
		if *csvflag {
			log.Fatalf("cannot use both -F and -csv")
		}
		re, err := regexp.Compile(*Fflag)
		if err != nil {
			log.Fatalf("parsing -F regexp: %v", err)
		}
		split = func(s string) []string { return re.Split(s, -1) }
	}

	if flag.NArg() == 0 {
		convert(os.Stdin)
	} else {
		for _, file := range flag.Args() {
			f, err := os.Open(file)
			if err != nil {
				log.Fatal(err)
			}
			convert(f)
			f.Close()
		}
	}
	var out []string
	for _, t := range total {
		if t.iok {
			out = append(out, fmt.Sprint(t.i))
		} else if t.fok {
			out = append(out, fmt.Sprint(t.f))
		} else {
			out = append(out, t.s)
		}
	}
	if *csvflag {
		w := csv.NewWriter(os.Stdout)
		w.Write(out)
		w.Flush()
		return
	}
	fmt.Printf("%s\n", strings.Join(out, " "))
}

func convert(f *os.File) {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatalf("%s: reading: %v", f.Name(), err)
		return
	}
	if *csvflag {
		recs, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil {
			log.Fatalf("%s: decoding csv: %v", f.Name(), err)
		}
		for _, rec := range recs {
			do(rec)
		}
	} else {
		for len(data) > 0 {
			line, rest, _ := bytes.Cut(data, []byte("\n"))
			do(split(string(line)))
			data = rest
		}
	}
}

func do(fields []string) {
	for ix, s := range fields {
		i, ierr := strconv.ParseInt(s, 0, 64)
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil && ierr == nil {
			f, ferr = float64(i), nil
		}
		switch {
		case ferr != nil && ix >= len(total):
			total = append(total, out{s: s})

		case ferr != nil:
			total[ix].fok = false
			total[ix].iok = false
			if total[ix].s != s {
				total[ix].s = "~"
			}

		case ix >= len(total):
			total = append(total, out{f: f, i: i, fok: true, iok: ierr == nil})

		case total[ix].fok:
			total[ix].f += f
			total[ix].i += i
			if ierr != nil {
				total[ix].iok = false
			}

		case total[ix].s == s:
			// keep it

		default:
			total[ix].s = "~"
		}
	}
}
