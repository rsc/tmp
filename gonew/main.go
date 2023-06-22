// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gonew starts a new Go module by copying a template module.
//
// Usage:
//
//	gonew srcMod[@version] [dstMod [dir]]
//
// Gonew makes a copy of the srcMod, changing its module path to dstMod.
// It writes that new to a new directory named by dir.
// If dir already exists it must be an empty directory.
// If dir is omitted, gonew uses ./elem where elem is the final path element of dstMod.
//
// This command is highly experimental and subject to change.
//
// Example
//
// To clone the basic command-line program template rsc.io/tmp/newcmd
// as your.domain/myprog, in the directory ./myprog:
//
//	gonew rsc.io/tmp/newcmd your.domain/myprog
//
// Or without having to install gonew first:
//
//	go run rsc.io/tmp/gonew@latest rsc.io/tmp/newcmd your.domain/myprog
//
// To clone a module without renaming the module:
//
//	gonew rsc.io/tmp/quote
//
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

func usage() {
	fmt.Fprintf(os.Stderr, "gonew srcMod[@version] [dstMod [dir]]\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("gonew: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 || len(args) > 3 {
		usage()
	}

	srcMod := args[0]
	srcModVers := srcMod
	if !strings.Contains(srcModVers, "@") {
		srcModVers += "@latest"
	}
	srcMod, _, _ = strings.Cut(srcMod, "@")
	srcBase := path.Base(srcMod)

	dstMod := srcMod
	if len(args) >= 2 {
		dstMod = args[1]
	}
	dstBase := path.Base(dstMod)

	var dir string
	if len(args) == 3 {
		dir = args[2]
	} else {
		dir = "." + string(filepath.Separator) + dstBase
	}

	// Dir must not exist or must be an empty directory.
	de, err := os.ReadDir(dir)
	if err == nil && len(de) > 0 {
		log.Fatalf("target directory %s exists and is non-empty", dir)
	}
	needMkdir := err != nil

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "mod", "download", "-json", srcModVers)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("go mod download -json %s: %v\n%s%s", srcModVers, err, stderr.Bytes(), stdout.Bytes())
	}

	var info struct {
		Dir string
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		log.Fatalf("go mod download -json %s: invalid JSON output: %v\n%s%s", srcMod, err, stderr.Bytes(), stdout.Bytes())
	}

	if needMkdir {
		if err := os.MkdirAll(dir, 0777); err != nil {
			log.Fatal(err)
		}
	}

	// Replace srcMod -> dstMod in go.mod file module line and imports.
	r := strings.NewReplacer(
		"module "+srcMod+"\n", "module "+dstMod+"\n",
		`"`+srcMod+`"`, `"`+dstMod+`"`,
		`"`+srcMod+`/`, `"`+dstMod+`/`,
	)

	filepath.WalkDir(info.Dir, func(src string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		rel := strings.Trim(strings.TrimPrefix(src, info.Dir), string(filepath.Separator))
		dst := filepath.Join(dir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dst, 0777); err != nil {
				log.Fatal(err)
			}
			return nil
		}

		data, err := os.ReadFile(src)
		if err != nil {
			log.Fatal(err)
		}
		var buf bytes.Buffer
		old := string(data)
		if !strings.Contains(rel, string(filepath.Separator)) {
			old = strings.ReplaceAll(old, "package "+srcBase+" //", "package "+dstBase+" //")
			old = strings.ReplaceAll(old, "package "+srcBase+"\n", "package "+dstBase+"\n")
		}
		r.WriteString(&buf, old)
		if err := os.WriteFile(dst, buf.Bytes(), 0666); err != nil {
			log.Fatal(err)
		}
		return nil
	})

	log.Printf("initialized %s in %s", dstMod, dir)
}
