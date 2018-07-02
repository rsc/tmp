// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Diffmod identifies differences in the dependencies
// implied by each of a set of go.mod files.
//
// Usage:
//
//	diffmod go.mod other/go.mod ...
//
// If the version of any dependency used by one go.mod file
// is different from the version used by another go.mod file
// (ignoring those that don't use the dependency at all),
// then diffmod prints a stanza of the form:
//
//	module/path
//		go.mod: version used in go.mod
//		other/go.mod: version used in other/go.mod
//		...
//
// Diffmod prints one stanza for each dependency that differs
// across the set of go.mod files.
//
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: diffmod go.mod other/go.mod...\n")
	os.Exit(2)
}

var deps = make(map[string]map[string]string) // go.mod -> module path -> Module

type Module struct {
	Path    string
	Main    bool
	Version string
	Replace Replacement
}

func (m Module) VersionString() string {
	var s string
	if m.Version != "" {
		s += " " + m.Version
	}
	if m.Replace.Dir != "" {
		s += " => " + m.Replace.Dir
	} else if m.Replace.Path != "" {
		s += " => " + m.Replace.Path + " " + m.Replace.Version
	}
	if s == "" {
		return ""
	}
	return s[1:]
}

type Replacement struct {
	Path    string
	Version string
	Dir     string
}

func main() {
	log.SetPrefix("syncmod: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	gomods := flag.Args()
	if len(gomods) < 2 {
		usage()
	}

	havePath := make(map[string]bool)
	var paths []string

	for _, gomod := range flag.Args() {
		if filepath.Base(gomod) != "go.mod" {
			log.Fatalf("not a go.mod: %s", gomod)
		}
		deps[gomod] = make(map[string]string)
		cmd := exec.Command("vgo", "list", "-m", "-json", "all")
		cmd.Dir = filepath.Dir(gomod)
		out, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(out))
		for {
			var m Module
			err := dec.Decode(&m)
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("%s: parsing json: %v", gomod, err)
			}
			if m.Main {
				continue
			}
			if !havePath[m.Path] {
				havePath[m.Path] = true
				paths = append(paths, m.Path)
			}
			deps[gomod][m.Path] = m.VersionString()
		}
	}

	sort.Strings(paths)
	for _, path := range paths {
		ok := true
		var m1 string
		for _, gomod := range gomods {
			m := deps[gomod][path]
			if m1 == "" {
				m1 = m
			}
			if m != "" && m != m1 {
				ok = false
			}
		}
		if ok {
			continue
		}
		fmt.Printf("%s\n", path)
		for _, gomod := range gomods {
			m := deps[gomod][path]
			if m != "" {
				fmt.Printf("\t%s: %s\n", gomod, m)
			}
		}
	}
}
