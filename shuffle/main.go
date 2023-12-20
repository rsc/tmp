// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Shuffle randomly permutes its input lines.
//
// Usage:
//
//	shuffle [-b] [-g regexp] [-m max] [file...]
//
// Shuffle reads the named files, or else standard input
// and then prints a random permutation of the input lines.
//
// The -b flag causes shuffle to shuffle blocks of non-blank lines
// in the input (separated by blank lines) rather than individual lines.
//
// The -g flag only shuffles lines or blocks matching the regexp.
//
// The -m flag specifies the maximum number of lines (or blocks) to print.
// When -m is given, shuffle requires memory only for the output,
// not for the entire input.
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
)

var (
	max   = flag.Int("m", 0, "print at most `max` lines (or blocks)")
	block = flag.Bool("b", false, "shuffle blank-line-separated blocks")
	grep  = flag.String("g", "", "consider only lines (or blocks) matching `regexp`")

	grepRE *regexp.Regexp
)

func main() {
	flag.Parse()
	if *grep != "" {
		re, err := regexp.Compile(*grep)
		if err != nil {
			log.Fatal(err)
		}
		grepRE = re
	}
	if flag.NArg() == 0 {
		collect(os.Stdin)
	} else {
		for _, file := range flag.Args() {
			f, err := os.Open(file)
			if err != nil {
				log.Fatal(err)
			}
			collect(f)
			f.Close()
		}
	}
	show()
}

var list []string
var n int

func add(s string) {
	n++
	i := rand.Intn(n)
	if *max == 0 || len(list) < *max {
		list = append(list, s)
		list[i], list[n-1] = list[n-1], list[i]
	} else if i < *max {
		list[i] = s
	}
}

func show() {
	for i, s := range list {
		if *block && i > 0 {
			os.Stdout.WriteString("\n")
		}
		os.Stdout.WriteString(s)
	}
}

func read1(b *bufio.Reader) string {
	s, err := b.ReadString('\n')
	if err == io.EOF && s != "" {
		s += "\n"
		err = nil
	}
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	if s == "" {
		return ""
	}
	isBlank := true
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
			isBlank = false
			break
		}
	}
	if isBlank {
		return "\n"
	}
	return s
}

func collect(r io.Reader) {
	b := bufio.NewReader(r)
	for {
		var s string
		if *block {
			var lines []string
			for {
				s := read1(b)
				if s == "\n" || s == "" {
					if len(lines) == 0 {
						if s == "" {
							return
						}
						continue
					}
					break
				}
				lines = append(lines, s)
			}
			s = strings.Join(lines, "")
		} else {
			s = read1(b)
			if s == "" {
				return
			}
		}
		if grepRE == nil || grepRE.MatchString(s) {
			add(s)
		}
	}
}

func addNL(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return data
}
