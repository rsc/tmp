// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Box is a simple sandbox for running untrusted programs.
// It is not necessarily that difficult to break out of, but it helps.
package main

/*
#cgo CFLAGS: -Wno-deprecated-declarations

#include <sandbox.h>
*/
import "C"
import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

var printPolicy = flag.Bool("print-policy", false, "print sandbox policy to stderr")

func usage() {
	fmt.Fprintf(os.Stderr, "box [options] cmd [args...]\n")
	os.Exit(1)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("box: ")
	flag.Usage = usage
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	os.Setenv("BOXROOT", wd)
	environ := os.Environ()

	r := newRules()
	r.addFiles()
	r.emitFileRules()
	profile := r.text.String()
	if *printPolicy {
		os.Stderr.WriteString(profile)
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}
	prog, err := exec.LookPath(args[0])
	if err != nil {
		log.Fatal(err)
	}

	var errstr *C.char
	if C.sandbox_init(C.CString(profile), 0, &errstr) != 0 {
		log.Fatalf("sandbox_init: %v", C.GoString(errstr))
	}

	err = syscall.Exec(prog, args, environ)
	log.Fatalf("exec %s: %v", args[0], err)
}
