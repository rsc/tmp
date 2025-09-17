// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"9fans.net/go/acme"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: atalk file.talk\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("aslide: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}
	file := flag.Arg(0)

	w, err := acme.New()
	if err != nil {
		log.Fatal(err)
	}
	w.Name("/talk/" + strings.TrimSuffix(filepath.Base(file), ".talk"))
	w.Write("tag", []byte(" − +"))

	var slides [][]byte
	reload := func() {
		data, err := os.ReadFile(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}

		data = append([]byte("\n"), data...)
		slides = bytes.Split(data, []byte("\n#\n"))
		for lineno, line := range strings.Split(string(slides[0]), "\n") {
			f := strings.Fields(line)
			if len(f) == 0 {
				continue
			}
			if len(f) >= 2 && f[0] == "Font" {
				w.Ctl("font " + f[1])
				continue
			}
			w.Errf("%s:%d: unknown directive: %s", file, lineno+1, line)
		}
		slides = slides[1:]
		for len(slides) > 0 && len(bytes.TrimSpace(slides[len(slides)-1])) == 0 {
			slides = slides[:len(slides)-1]
		}
		if len(slides) == 0 {
			log.Fatal("no slides in file")
		}
	}
	reload()

	slideNum := 0
	show := func() {
		if slideNum >= len(slides) {
			slideNum = len(slides) - 1
		}
		if slideNum < 0 {
			slideNum = 0
		}
		w.Addr(",")
		var slide []byte
		if len(slides) > 0 {
			slide = slides[slideNum]
		}
		slide = append([]byte{'\n'}, slide...)
		slide = bytes.ReplaceAll(slide, []byte("\n"), []byte("\n\t"))
		slide = append(slide, '\n')
		w.Write("data", slide)
		w.Addr("0")
		w.Ctl("dot=addr")
		w.Ctl("show")
		w.Addr("$")
		w.Ctl("dot=addr")
		w.Ctl("clean")
	}
	show()

	for e := range w.EventChan() {
		switch e.C2 {
		case 'x', 'X': // execute
			switch string(e.Text) {
			default:
				w.WriteEvent(e)

			case "Del":
				w.Ctl("delete")

			case "Get":
				reload()
				show()

			case "Edit":
				if err := exec.Command("B", file).Run(); err != nil {
					w.Errf("B %s: %v\n", file, err)
				}

			case "+":
				if slideNum+1 < len(slides) {
					slideNum++
					show()
				}

			case "-", "−":
				if slideNum > 0 {
					slideNum--
					show()
				}
			}
		}
	}
}
