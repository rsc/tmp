// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Dirhash computes a hash of a file system directory tree.
//
// Usage:
//
//	dirhash [-d] [dir ...]
//
// For each directory named on the command line, dirhash prints
// the hash of the file system tree rooted at that directory.
//
// The hash is computed by considering all files in the tree,
// in the lexical order used by Go's filepath.Walk, computing
// the sha256 hash of each, and then computing a sha256 of
// the list of hashes and file names. If the -d flag is given,
// dirhash prints to standard error a shell script computing
// the overall sha256.
//
// Except for occasional differences in sort order, "dirhash mydir"
// is equivalent to
//
//	(cd mydir; sha256sum $(find . -type f | sort) | sha256sum)
//
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: dirhash [-d] [dir...]\n")
	os.Exit(2)
}

var debug = flag.Bool("d", false, "print input for overall sha256sum")

func main() {
	log.SetFlags(0)
	log.SetPrefix("dirhash: ")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}

	for _, dir := range args {
		dirhash(dir)
	}
}

func dirhash(dir string) {
	dir = filepath.Clean(dir)
	h := sha256.New()
	info, err := os.Lstat(dir)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		log.Printf("%s is a symlink\n", dir)
		return
	}
	if *debug {
		fmt.Fprintf(os.Stderr, "sha256sum << 'EOF'\n")
	}
	filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if info.Mode()&os.ModeSymlink != 0 {
			i, err := os.Stat(file)
			if err != nil {
				return err
			}
			info = i
		}
		if info.IsDir() {
			return nil
		}
		rel := file
		if dir != "." {
			rel = file[len(dir)+1:]
		}
		rel = filepath.ToSlash(rel)
		fh := filehash(file)
		if *debug {
			fmt.Fprintf(os.Stderr, "%s  ./%s\n", fh, rel)
		}
		fmt.Fprintf(h, "%s  ./%s\n", fh, rel)
		return nil
	})
	if *debug {
		fmt.Fprintf(os.Stderr, "EOF\n")
	}
	fmt.Printf("%x %s\n", h.Sum(nil), dir)
}

func filehash(file string) string {
	h := sha256.New()
	f, err := os.Open(file)
	if err != nil {
		log.Print(err)
	}
	_, err = io.Copy(h, f)
	if err != nil {
		log.Print(err)
	}
	f.Close()
	return fmt.Sprintf("%x", h.Sum(nil))
}
