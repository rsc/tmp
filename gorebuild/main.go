// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Work in progress. DO NOT USE.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"
)

var pFlag = flag.Int("p", 1, "run `n` builds in parallel")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: gorebuild [goos-goarch][@version]...\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("gorebuild: ")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	// Undocumented feature for developers working on report template:
	// pass in a gorebuild.json file and it reformats the gorebuild.html file.
	if len(args) == 1 && strings.HasSuffix(args[0], ".json") {
		reformat(args[0])
		return
	}

	r := Run(flag.Args())
	writeJSON(r)
	writeHTML(r)
	if r.Log.Status != PASS {
		os.Exit(3)
	}
}

func reformat(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		log.Fatal(err)
	}
	writeHTML(&r)
}

func writeJSON(r *Report) {
	js, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		log.Fatal(err)
	}
	js = append(js, '\n')
	if err := os.WriteFile("gorebuild.json", js, 0666); err != nil {
		log.Fatal(err)
	}
}

//go:embed report.tmpl
var reportTmpl string

func writeHTML(r *Report) {
	t, err := template.New("report.tmpl").Parse(reportTmpl)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, &r); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("gorebuild.html", buf.Bytes(), 0666); err != nil {
		log.Fatal(err)
	}
}
