// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Palm is an interactive client for [Google's PaLM API].
//
// Usage:
//
//	palm [-l] [-k keyfile] [prompt...]
//
// Palm concatenates its arguments, sends the result as a prompt
// to the PaLM model, and prints the response.
//
// With no arguments, palm reads standard input until EOF
// and uses that as the prompt.
//
// The -l flag runs palm in an interactive line-based mode:
// it reads a single line of input and prints the PaLM response,
// and repeats. The -l flag cannot be used with arguments.
//
// The -k flag specifies the name of a file containing the PaLM API key
// (default $HOME/.palmkey).
//
// [Google's PaLM API]: https://developers.generativeai.google/
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	home, _  = os.UserHomeDir()
	key      string
	lineMode = flag.Bool("l", false, "line at a time mode")
	keyFile  = flag.String("k", filepath.Join(home, ".palmkey"), "read palm API key from `file`")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: palm [-l] [-k keyfile]\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("palm: ")
	flag.Usage = usage
	flag.Parse()

	data, err := os.ReadFile(*keyFile)
	if err != nil {
		log.Fatal(err)
	}
	key = strings.TrimSpace(string(data))

	if *lineMode {
		if flag.NArg() != 0 {
			log.Fatalf("-l cannot be used with arguments")
		}
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Fprintf(os.Stderr, "> ")
			if !scanner.Scan() {
				break
			}
			line := scanner.Text()
			fmt.Fprintf(os.Stderr, "\n")
			do(line)
			fmt.Fprintf(os.Stderr, "\n")
		}
		return
	}

	if flag.NArg() != 0 {
		do(strings.Join(flag.Args(), " "))
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		do(string(data))
	}
}

func do(prompt string) {
	// curl \
	// -H 'Content-Type: application/json' \
	// -d '{ "prompt": { "text": "Write a story about a magic backpack"} }' \
	// "https://generativelanguage.googleapis.com/v1beta3/models/text-bison-001:generateText?key=YOUR_API_KEY"

	js, err := json.Marshal(map[string]map[string]string{"prompt": {"text": prompt}})
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.Post("https://generativelanguage.googleapis.com/v1beta3/models/text-bison-001:generateText?key="+key, "application/json", bytes.NewReader(js))
	if err != nil {
		log.Fatal(err)
	}
	if err != nil {
		log.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("%s:\n%s", resp.Status, data)
	}
	if err != nil {
		log.Fatalf("reading body: %v", err)
	}

	var r Response
	if err := json.Unmarshal(data, &r); err != nil {
		log.Fatal(err)
	}
	if len(r.Candidates) == 0 {
		fmt.Fprintf(os.Stderr, "no candidate answers")
	}
	for _, c := range r.Candidates {
		fmt.Printf("%s\n", c.Output)
		for _, rate := range c.SafetyRatings {
			if rate.Probability != "NEGLIGIBLE" {
				fmt.Printf("%s=%s\n", rate.Category, rate.Probability)
			}
		}
	}
}

type Response struct {
	Candidates []Candidate
}

type Candidate struct {
	Output        string
	SafetyRatings []SafetyRating
}

type SafetyRating struct {
	Category    string
	Probability string
}
