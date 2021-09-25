// Copyright 2021 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Yaml2json converts YAML input to JSON output.
//
// Usage:
//
//	yaml2json [-o output] [file...]
//
// Yaml2json reads the named files, or else standard input, as YAML input
// and prints that data in JSON form to standard output.
//
// The -o flag specifies the name of a file to write instead of using standard output.
//
// Example
//
// To print a YAML file as JSON:
//
//	yaml2json data.yaml
//
// To convert one:
//
//	yaml2json -o data.json data.yaml
//
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	oflag = flag.String("o", "", "write output to `file` (default standard output)")

	output  *bufio.Writer
	comment rune
	exit    = 0
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: yaml2json [-o output] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("yaml2json: ")
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
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Print("%s: reading: %v", f.Name(), err)
		exit = 1
		return
	}
	var d interface{}
	if err := yaml.Unmarshal(data, &d); err != nil {
		log.Print("%s: decoding: %v", f.Name(), err)
		exit = 1
		return
	}
	data, err = json.MarshalIndent(&d, "", "\t")
	if err != nil {
		log.Print("%s: encoding: %v", f.Name(), err)
		exit = 1
		return
	}
	output.Write(data)
	output.WriteByte('\n')
}
