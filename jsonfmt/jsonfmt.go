// Copyright 2021 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Jsonfmt reformats JSON data.
//
// Usage:
//
//	jsonfmt [-f template] [-o output] [file...]
//
// Jsonfmt reads the named files, or else standard input, as JSON data
// and then reprints that same JSON data to standard output.
//
// The -f flag specifies a template to execute on the JSON data.
// The output of the template is printed to standard output
// instead of reformatted JSON.
//
// The -o flag specifies the name of a file to write instead of using standard output.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
)

var (
	fflag = flag.String("f", "", "use template to format JSON objects")
	oflag = flag.String("o", "", "write output to `file` (default standard output)")

	tmpl    *template.Template
	output  *bufio.Writer
	comment rune
	exit    = 0
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: jsonfmt [-o output] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("jsonfmt: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	outfile := os.Stdout
	if *oflag != "" {
		f, err := os.Create(*oflag)
		if err != nil {
			log.Fatal(err)
		}
		outfile = f
	}
	if *fflag != "" {
		t, err := template.New("").Parse(*fflag)
		if err != nil {
			log.Fatalf("parsing -f template: %v", err)
		}
		tmpl = t
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
	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("%s: reading: %v", f.Name(), err)
		exit = 1
		return
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tmpl == nil {
		dec.UseNumber()
	}
	for {
		var x any
		if err := dec.Decode(&x); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("%s: decoding: %v", f.Name(), err)
			exit = 1
			return
		}
		var out []byte
		if tmpl != nil {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, x); err != nil {
				log.Printf("%s: executing template: %v", f.Name(), err)
				exit = 1
				return
			}
			out = buf.Bytes()
		} else {
			js, err := json.Marshal(x)
			if err != nil {
				log.Printf("%s: remarshaling: %v", f.Name(), err)
				exit = 1
				return
			}
			out = js
		}
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		os.Stdout.Write(out)
	}
}
