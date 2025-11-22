// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Macpanic summarizes macOS panic logs.
//
// Usage:
//
//	macpanic [-k kernel] [file...]
//
// Macpanic reads each of the named panic logs and summarizes the panic.
// With no arguments it reads /Library/Logs/DiagnosticReports/Kernel*panic.
// To add symbol information to the panic summary, macpanic uses symbols
// from kernel (default /System/Library/Kernels/kernel) and also inspects
// installed kernel modules.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ianlancetaylor/demangle"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: macpanic [-k kernel] [file...]\n")
	os.Exit(2)
}

var kernel = flag.String("k", "/System/Library/Kernels/kernel", "kernel binary")
var version string

type sym struct {
	addr uint64
	name string
}

var syms []sym

func main() {
	log.SetPrefix("macpanic: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	data, err := os.ReadFile(*kernel)
	if err != nil {
		log.Fatal(err)
	}
	i := bytes.Index(data, []byte("Darwin Kernel Version"))
	if i < 0 {
		log.Fatalf("cannot find 'Darwin Kernel Version' in kernel")
	}
	data = data[i:]
	i = bytes.IndexByte(data, 0)
	if i < 0 || !utf8.Valid(data[:i]) {
		log.Fatalf("found malformed 'Darwin Kernel Version' in kernel")
	}
	version = string(data[:i])
	fmt.Printf("kernel %s: %s\n", *kernel, version)

	syms, err = nm(*kernel)
	if err != nil {
		log.Fatal(err)
	}

	args := flag.Args()
	if len(args) == 0 {
		list, err := filepath.Glob("/Library/Logs/DiagnosticReports/Kernel*panic")
		if err != nil {
			log.Fatal(err)
		}
		args = list
	}
	for _, arg := range args {
		process(arg)
	}
}

func nm(file string) ([]sym, error) {
	var syms []sym
	data, err := exec.Command("nm", file).Output()
	if err != nil {
		return nil, fmt.Errorf("nm %s: %v", file, err)
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		i := bytes.IndexByte(line, ' ')
		if i < 0 {
			continue
		}
		j := bytes.IndexByte(line[i+1:], ' ')
		if i < 0 {
			continue
		}
		j += i + 1
		addr, err := strconv.ParseUint(string(line[:i]), 16, 64)
		if err != nil {
			continue
		}
		name := string(line[j+1:])
		syms = append(syms, sym{addr, name})
	}
	sort.Slice(syms, func(i, j int) bool {
		return syms[i].addr < syms[j].addr
	})
	return syms, nil
}

func process(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		log.Print(err)
		return
	}

	i := bytes.Index(data, []byte("Kernel slide:"))
	if i < 0 {
		log.Printf("%s: cannot find kernel slide", file)
		return
	}
	j := bytes.IndexByte(data[i:], '\n')
	if j < 0 {
		log.Printf("%s: cannot find kernel slide", file)
		return
	}
	j += i

	s := strings.TrimSpace(string(data[i+len("Kernel slide:") : j]))
	slide, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		log.Printf("%s: cannot parse kernel slide %q", file, s)
		return
	}

	i = bytes.Index(data, []byte("Kernel text base:"))
	if i < 0 {
		log.Printf("%s: cannot find kernel slide", file)
		return
	}
	j = bytes.IndexByte(data[i:], '\n')
	if j < 0 {
		log.Printf("%s: cannot find kernel text base", file)
		return
	}
	j += i
	s = strings.TrimSpace(string(data[i+len("Kernel text base:") : j]))
	base, err := strconv.ParseUint(s, 0, 64)
	if err != nil {
		log.Printf("%s: cannot parse kernel text base %q", file, s)
		return
	}

	i = bytes.Index(data, []byte("Kernel version:\n"))
	if i < 0 {
		log.Printf("%s: cannot find kernel version", file)
		return
	}
	j = bytes.IndexByte(data[i+len("Kernel version:\n"):], '\n')
	if j < 0 {
		log.Printf("%s: cannot find kernel version", file)
		return
	}
	j += i + len("Kernel version:\n")
	v := string(data[i+len("Kernel version:\n") : j])
	if v != version {
		log.Printf("%s: mismatched kernel version %q != %q", file, v, version)
		return
	}

	i = bytes.Index(data, []byte("\npanic"))
	if i < 0 {
		log.Printf("%s: cannot find panic", file)
		return
	}
	i++
	j = bytes.Index(data[i:], []byte("\n"))
	if j < 0 {
		log.Printf("%s: cannot find panic", file)
		return
	}
	p := string(data[i : i+j])

	i = bytes.Index(data, []byte("\nBacktrace"))
	if i < 0 {
		log.Printf("%s: cannot find backtrace", file)
		return
	}

	var trace [][2]uint64
	var exts []sym
	lines := bytes.Split(data[i+1:], []byte("\n"))[1:]
	for len(lines) > 0 {
		line := strings.TrimSpace(string(lines[0]))
		if line == "" {
			break
		}
		lines = lines[1:]
		if strings.HasPrefix(line, "Kernel Extensions in backtrace") {
			for len(lines) > 0 {
				line := strings.TrimSpace(string(lines[0]))
				if line == "" {
					break
				}
				lines = lines[1:]
				i := strings.Index(line, "(")
				if i < 0 {
					break
				}
				j := strings.LastIndex(line, "@")
				if j < 0 {
					break
				}
				name := line[:i]
				addr := line[j+1:]
				if i := strings.Index(addr, "->"); i >= 0 {
					addr = addr[:i]
				}
				a, err := strconv.ParseUint(addr, 0, 64)
				if err != nil {
					log.Printf("%s: cannot parse extension address: %s", file, line)
					continue
				}
				exts = append(exts, sym{a, name})
			}
			break
		}
		i := strings.Index(line, " : ")
		if i < 0 {
			log.Printf("%s: cannot parse backtrace line: %s", file, line)
			break
		}
		a, err := strconv.ParseUint(line[:i], 0, 64)
		b, err1 := strconv.ParseUint(line[i+3:], 0, 64)
		if err != nil || err1 != nil {
			log.Printf("%s: cannot parse backtrace line: %s", file, line)
			break
		}
		trace = append(trace, [2]uint64{a, b})
	}

	sort.Slice(exts, func(i, j int) bool {
		return exts[i].addr < exts[j].addr
	})

	fmt.Printf("\n%s\n", file)
	fmt.Printf("\t%s\n", p)
	for _, t := range trace {
		var desc string
		if t[1] < base {
			desc = translate(t[1], exts, true)
		} else {
			desc = translate(t[1]-slide, syms, false)
		}
		fmt.Printf("\t%#x : %#x : %s\n", t[0], t[1], desc)
	}
}

func translate(pc uint64, syms []sym, exts bool) string {
	i := sort.Search(len(syms), func(i int) bool {
		return i+1 >= len(syms) || syms[i+1].addr > pc
	})
	if i >= len(syms) {
		return "???"
	}
	name := syms[i].name
	n, err := demangle.ToString(name)
	if err != nil {
		n, err = demangle.ToString(strings.TrimPrefix(name, "_"))
	}
	if err == nil {
		name = n
	}
	desc := fmt.Sprintf("%s + %#x", name, pc-syms[i].addr)
	if exts {
		name := strings.TrimSuffix(syms[i].name, ".kext")
		elem := name[strings.LastIndex(name, ".")+1:]
		esyms, err := nm("/System/Library/Extensions/" + elem + ".kext/Contents/MacOS/" + elem)
		if err != nil {
			esyms, err = nm("/Library/Extensions/" + elem + ".kext/Contents/MacOS/" + elem)
		}
		if err == nil {
			d := translate(pc-syms[i].addr, esyms, false)
			if d != "???" {
				desc += " (" + d + ")"
			}
		}
	}
	return desc
}
