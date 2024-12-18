// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Ivymark updates Ivy blocks in Markdown files.
//
// Usage:
//
//	ivymark [-w] [file...]
//
// Ivymark reads the named files, or else standard input, as Markdown documents,
// executes any Ivy code blocks and updates them to contain the results,
// and then reprints the Markdown documents to standard output .
//
// The -w flag specifies to rewrite the files in place.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"iter"
	"log"
	"os"
	"strings"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/parse"
	"robpike.io/ivy/run"
	"robpike.io/ivy/scan"
	"rsc.io/markdown"
)

var (
	htmlflag = flag.Bool("html", false, "write HTML output")
	wflag    = flag.Bool("w", false, "write output back to input files")
	exit     = 0
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: ivymark [-html] [-w] [file...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("ivymark: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		os.Stdin.Close() // stop ivy
		convert(data, "")
	} else {
		os.Stdin.Close() // stop ivy
		for _, file := range flag.Args() {
			data, err := os.ReadFile(file)
			if err != nil {
				log.Print(err)
				exit = 1
				continue
			}
			convert(data, file)
		}
	}
	os.Exit(exit)
}

func convert(data []byte, file string) {
	var p markdown.Parser
	p.Table = true
	doc := p.Parse(string(data))
	update(doc)
	var out []byte
	if *htmlflag {
		out = []byte(markdown.ToHTML(doc))
	} else {
		out = []byte(markdown.Format(doc))
	}
	if *wflag && file != "" {
		if err := os.WriteFile(file, out, 0666); err != nil {
			log.Print(err)
			exit = 1
			return
		}
	} else {
		os.Stdout.Write(out)
	}
}

func update(doc *markdown.Document) {
	var conf config.Config
	var outBuf, errBuf bytes.Buffer
	conf.SetFormat("")
	conf.SetMaxBits(1e6)
	conf.SetMaxDigits(1e4)
	conf.SetMaxStack(100000)
	conf.SetOrigin(1)
	conf.SetPrompt("")
	conf.SetOutput(&outBuf)
	conf.SetErrOutput(&errBuf)

	context := exec.NewContext(&conf)

	for code := range codeBlocks(doc) {
		text := strings.Join(code.Text, "\n")
		text, _, _ = strings.Cut(text, "\n-- err --\n")
		text, _, _ = strings.Cut(text, "\n-- out --\n")
		text = addNL(text)
		if text != "" {
			scanner := scan.New(context, "input", strings.NewReader(text))
			parser := parse.NewParser("input", scanner, context)
			outBuf.Reset()
			errBuf.Reset()
			run.Run(parser, context, false)
			if out := addNL(outBuf.String()); out != "" {
				text += "-- out --\n" + out
			}
			if err := addNL(errBuf.String()); err != "" {
				text += "-- err --\n" + err
			}
		}
		lines := strings.Split(text, "\n")
		lines = lines[:len(lines)-1] // remove empty line after last \n
		code.Text = lines
	}
}

func addNL(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return s
	}
	return s + "\n"
}

func codeBlocks(doc *markdown.Document) iter.Seq[*markdown.CodeBlock] {
	return func(yield func(*markdown.CodeBlock) bool) {
		walk(doc, yield)
	}
}

func walk(b markdown.Block, yield func(*markdown.CodeBlock) bool) bool {
	switch b := b.(type) {
	case *markdown.CodeBlock:
		if !yield(b) {
			return false
		}
	case *markdown.Document:
		for _, bb := range b.Blocks {
			if !walk(bb, yield) {
				return false
			}
		}
	case *markdown.List:
		for _, bb := range b.Items {
			if !walk(bb, yield) {
				return false
			}
		}
	case *markdown.Item:
		for _, bb := range b.Blocks {
			if !walk(bb, yield) {
				return false
			}
		}
	case *markdown.Quote:
		for _, bb := range b.Blocks {
			if !walk(bb, yield) {
				return false
			}
		}
	}
	return true
}
