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
	"strconv"
	"strings"
	"syscall"
)

func usage() {
	fmt.Fprintf(os.Stderr, "box [options] cmd [args...]\n")
	os.Exit(1)
}

//go:embed default.sb
var default_sb string

func main() {
	log.SetFlags(0)
	log.SetPrefix("box: ")
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	os.Setenv("BOXROOT", wd)

	prog, err := exec.LookPath(args[0])
	if err != nil {
		log.Fatal(err)
	}

	profile := default_sb

	environ := os.Environ()
	var envs strings.Builder
	fmt.Fprintf(&envs, "(define *env* '(")
	for _, env := range environ {
		key, val, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		fmt.Fprintf(&envs, "(%s . %s) ", strconv.Quote(key), strconv.Quote(val))
	}
	fmt.Fprintf(&envs, "))\n")

	profile = strings.ReplaceAll(profile, "; SET ENVIRONMENT HERE\n", envs.String())

	var errstr *C.char
	if C.sandbox_init(C.CString(profile), 0, &errstr) != 0 {
		log.Fatalf("sandbox_init: %v", C.GoString(errstr))
	}

	err = syscall.Exec(prog, args, environ)
	log.Fatalf("exec %s: %v", args[0], err)
}
