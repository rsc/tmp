// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var chop = flag.Bool("chop", false, "show pieces on stdout and exit")
var cmdstr = flag.String("cmd", "", "command to run")
var cmd []string

func usage() {
	fmt.Fprintf(os.Stderr, "usage: dredge -cmd 'command args' files...\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("dredge: ")

	flag.Usage = usage
	flag.Parse()

	if !*chop {
		cmd = strings.Fields(*cmdstr)
		if len(cmd) == 0 || flag.NArg() == 0 {
			usage()
		}

		out, err := runCmd()
		if err != nil {
			log.Fatalf("command failed before any changes: %s\n%s", err, out)
		}
	}

	for {
		progress := false
		for _, file := range flag.Args() {
			progress = dredgeFile(file) || progress
		}
		if !progress {
			break
		}
	}
}

var cblockRE = regexp.MustCompile(`(?m)^([^/{ \t\n][^/{\n]*\n)*([^/{ \t\n][^/{\n]*)?{[ \t]*\n(([^}\n].*)?\n)*};?[ \t]*\n`)

func dredgeFile(file string) bool {
	fmt.Printf("%s\n", file)
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	matches := cblockRE.FindAllIndex(data, -1)
	if *chop {
		for _, m := range matches {
			fmt.Printf("«\n%s»\n", data[m[0]:m[1]])
		}
		return false
	}

	progress := false
	for {
		loopProgress := false
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			new := append(data[:m[0]:m[0]], data[m[1]:]...)
			if err := ioutil.WriteFile(file, new, 0666); err != nil {
				log.Fatal(err)
			}
			if _, err := runCmd(); err == nil {
				fmt.Printf("+")
				loopProgress = true
				progress = true
				data = new
			}
		}
		if !loopProgress {
			break
		}
		matches = cblockRE.FindAllIndex(data, -1)
	}
	ioutil.WriteFile(file, data, 0666) // always; must at least restore original
	return progress
}

func runCmd() (out []byte, err error) {
	fmt.Printf(".")
	return exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
}
