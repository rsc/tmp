// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Csv2tsv converts comma-separated value (CSV) input to tab-separated value (TSV) output.
//
// Usage:
//
//	csv2tsv [-c comment] [-o output] [-t tab] [file...]
//
// Csv2tsv reads the named files, or else standard input, as comma-separated value data
// and prints that data in tab-separated form to standard output.
//
// The -c flag specifies a comment character. Input lines beginning with this
// character will be elided.
//
// The -o flag specifies the name of a file to write instead of using standard output.
//
// The -t flag specifies a string to use in place of the tab character.
//
// Before printing the data, csv2tsv replaces every newline or occurrence of the tab string
// with a single space.
//
// Example
//
// To print the second and fourth fields of a CSV file using awk:
//
//	csv2tsv data.csv | awk -F'\t' '{print $2, $4}'
//
package main // import "rsc.io/csv2tsv"

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var (
	cflag = flag.String("c", "", "treat lines beginning with `char` as comments")
	oflag = flag.String("o", "", "write output to `file` (default standard output)")
	tab   = flag.String("t", "", "use `string` in place of tab in output")

	output  *bufio.Writer
	comment rune
	exit    = 0
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: csv2tsv [-o output] [-t tab] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("csv2tsv: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if *tab == "" {
		*tab = "\t"
	}

	if *cflag != "" {
		r := []rune(*cflag)
		if len(r) != 1 {
			log.Fatal("comment char %q must be a single rune", *cflag)
		}
		comment = r[0]
	}

	outfile := os.Stdout
	if *oflag != "" {
		f, err := os.Create(*oflag)
		if err != nil {
			log.Fatal(err)
		}
		outfile = f
	}
	output = bufio.NewWriter(outfile)

	if flag.NArg() == 0 {
		convert(os.Stdin)
	} else {
		for _, file := range flag.Args() {
			f, err := os.Open(file)
			if err != nil {
				log.Print(err)
				exit = 1
				continue
			}
			convert(f)
			f.Close()
		}
	}
	output.Flush()
	os.Exit(exit)
}

func convert(f *os.File) {
	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1
	r.Comment = comment
	for {
		rec, err := r.Read()
		if err != nil {
			if err != io.EOF {
				log.Print("reading %s: %v", f.Name(), err)
				exit = 1
			}
			break
		}
		for i, r := range rec {
			if i > 0 {
				output.WriteString(*tab)
			}
			r = strings.Replace(r, "\n", " ", -1)
			r = strings.Replace(r, *tab, " ", -1)
			output.WriteString(r)
		}
		output.WriteString("\n")
	}
}
