// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Uncover prints the code not covered in a coverage profile.
//
// Usage:
//
//	go test -coverprofile=c.out
//	uncover [-l] c.out
//
// Uncover prints a sequence of blocks, one for each uncovered
// section of code. Each block consists of a file address on a line
// by itself followed by the text of the code on those lines.
//
// By default, uncover prints file names relative to the current
// directory when appropriate. The -l flag forces it to print absolute (long)
// file names.
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var longNames = flag.Bool("l", false, "print long file names")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: uncover [-l] c.out\n")
	os.Exit(2)
}

var pwd string

func main() {
	log.SetPrefix("uncover: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}

	pwd, _ = os.Getwd()

	out, err := uncover(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.Write(out)
}

// uncover reads the profile data from profile
// and generates and returns an uncoverage report.
func uncover(profile string) ([]byte, error) {
	profiles, err := ParseProfiles(profile)
	if err != nil {
		return nil, err
	}
	dirs, err := findPkgs(profiles)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	for _, profile := range profiles {
		fn := profile.FileName
		file, err := findFile(dirs, fn)
		if err != nil {
			return nil, err
		}
		src, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("can't read %q: %v", fn, err)
		}
		err = uncoverFile(&buf, file, src, profile.Boundaries(src))
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// uncoverFile writes an uncoverage report for the given file to buf.
func uncoverFile(buf *bytes.Buffer, file string, src []byte, bounds []Boundary) error {
	if !*longNames && pwd != "" {
		rel, err := filepath.Rel(pwd, file)
		if err == nil && !strings.HasPrefix(rel, "..") {
			file = rel
		}
	}

	lastOffset, lastLine := 0, 1 // byte 0 is the start of line 1

	// findLine returns the line number and start and end offsets of the line containing p.
	findLine := func(p int) (line, lo, hi int) {
		// Find boundaries of line containing p.
		lo = p
		for lo > 0 && src[lo-1] != '\n' {
			lo--
		}
		hi = p
		for hi < len(src) && src[hi] != '\n' {
			hi++
		}
		if hi < len(src) {
			hi++
		}

		// Compute line number.
		// Count from previous position to avoid O(n²) behavior in huge files.
		if lo < lastOffset {
			lastOffset, lastLine = 0, 1
		}
		line = lastLine + bytes.Count(src[lastOffset:lo], []byte("\n"))
		return line, lo, hi
	}

	for i := 0; i+1 < len(bounds); i += 2 {
		start := bounds[i]
		end := bounds[i+1]
		if !start.Start || end.Start {
			return fmt.Errorf("boundaries out of sync in profile")
		}
		if start.Count > 0 {
			continue
		}

		startLine, startLo, startHi := findLine(start.Offset)
		startSimple := false
		if unimportant(src[start.Offset:startHi]) {
			// First line fragment is unimportant - skip to next line.
			startLo = startHi
			start.Offset = startLo
			startLine++
			startSimple = true
		} else if unimportant(src[startLo:start.Offset]) {
			startSimple = true
		}

		endLine, endLo, endHi := findLine(end.Offset)
		endSimple := false
		if unimportant(src[endLo:end.Offset]) {
			// Last line fragment is unimportant - back up to start of line.
			endHi = endLo
			end.Offset = endHi
			endLine--
			endSimple = true
		} else if unimportant(src[end.Offset:endHi]) {
			endSimple = true
		}

		if startSimple && endSimple && startLine > endLine {
			startLine = endLine
		}

		snippet := reindent(src[startLo:endHi])

		var addr string
		switch {
		case startSimple && endSimple && startLine == endLine:
			addr = fmt.Sprintf("%d", startLine)
		case startSimple && endSimple:
			addr = fmt.Sprintf("%d,%d", startLine, endLine)
		default:
			addr = fmt.Sprintf("%d:%d,%d:%d",
				startLine, 1+utf8.RuneCount(src[startLo:start.Offset]),
				endLine, 1+utf8.RuneCount(src[endLo:end.Offset]))
		}

		fmt.Fprintf(buf, "%s:%s\n", file, addr)
		buf.Write(snippet)

		if src[endHi-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	return nil
}

func unimportant(s []byte) bool {
	for _, b := range s {
		switch b {
		case ' ', '\t', '\n', '{', '}':
			// unimportant, keep looking
		default:
			return false
		}
	}
	return true
}

func reindent(src []byte) []byte {
	lines := bytes.SplitAfter(src, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	var prefix []byte
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			lines[i] = nil
			continue
		}
		if prefix == nil {
			j := 0
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			prefix = line[:j]
		}
		j := 0
		for j < len(prefix) && j < len(line) && line[j] == prefix[j] {
			j++
		}
		prefix = prefix[:j]
	}
	for i, line := range lines {
		if line == nil {
			lines[i] = []byte("\n")
			continue
		}
		lines[i] = append([]byte("\t"), line[len(prefix):]...)
	}
	src = bytes.Join(lines, nil)
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(src, '\n')
	}
	return src
}
