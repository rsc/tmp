// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Signre2 creates a signed tag for a new RE2 release.
//
// Usage:
//
//	signre2 [-delete] tag
//
// The -delete flag causes signre2 to start by deleting the remote tag with the given name
// (only for testing or recovering from earlier failures).
//
// Example:
//
//	signre2 2025-07-22
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var delete = flag.Bool("delete", false, "start by deleting remote tag (dangerous!)")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: signre2 [-delete] tag\n")
	os.Exit(1)
}

var tagobjRE = regexp.MustCompile(`(?s)\A(.*?tagger [^\n]+ )([0-9]+)( [+-]?[0-9]+\n.*\n)-----BEGIN SSH`)

func main() {
	log.SetPrefix("signre2: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 1 {
		usage()
	}
	tag := flag.Arg(0)

	os.RemoveAll("/tmp/re2")
	os.Mkdir("/tmp/re2", 0777)
	run("git", "clone", "https://code.googlesource.com/re2", ".")

	if *delete {
		gerrit("DELETE", "https://code-review.googlesource.com/a/projects/re2/tags/"+tag, nil, nil)
	}

	var br branchReply
	gerrit("GET", "https://code-review.googlesource.com/a/projects/re2/branches/main", nil, &br)
	log.Printf("main=%s", br.Revision)

	delta := int64(-1)
	prefix := ""
	suffix := ""
	sig := "-----BEGIN SSH SIGNATURE-----\nNot a signature.\n-----END SSH SIGNATURE-----\n"
	signed := ""
	target := time.Now().Add(2 * time.Second)
	for range 5 {
		var ta tagArgs
		var tr tagReply
		ta.Revision = br.Revision
		ta.Message = tag + "\n" + sig
		time.Sleep(time.Until(target))
		gerrit("PUT", "https://code-review.googlesource.com/a/projects/re2/tags/"+tag, &ta, &tr)

		// Fetch tag to local repo.
		runErr("git", "tag", "-d", tag)
		run("git", "pull", "-f", "--tags", "https://code.googlesource.com/re2", "refs/tags/"+tag)

		// Extract actual signature time.
		out := run("git", "cat-file", "-p", "refs/tags/"+tag)
		m := tagobjRE.FindStringSubmatch(string(out))
		if m == nil {
			log.Fatalf("unexpected tag format:\n%s", out)
		}
		if signed == m[1]+m[2]+m[3] {
			log.Printf("success!\n")
			break
		}
		gerrit("DELETE", "https://code-review.googlesource.com/a/projects/re2/tags/"+tag, nil, nil)
		i, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			log.Fatalf("bad tag time %s in:\n%s", m[2], out)
		}
		if prefix != "" && (m[1] != prefix || m[3] != suffix) {
			log.Fatalf("unexpected prefix/suffix in:\n%s\n\nexpected %s\n", out, signed)
		}
		delta = i - target.Unix()
		log.Printf("signing delta=%ds", delta)

		prefix = m[1]
		suffix = m[3]
		for {
			target = time.Now().Add(10 * time.Second)
			signed = fmt.Sprintf("%s%d%s", prefix, target.Unix()+delta, suffix)
			log.Print("signing:\n", signed)
			sig = string(runIn([]byte(signed), "ssh-keygen", "-Y", "sign", "-n", "git", "-f", os.Getenv("HOME")+"/.ssh/re2_signing_key"))
			if time.Until(target) > 1*time.Second {
				break
			}
		}
	}
	cmd := exec.Command("git", "verify-tag", tag)
	cmd.Dir = "/tmp/re2"
	out, err := cmd.CombinedOutput() // want stderr
	if err != nil {
		log.Fatalf("git verify-tag %s: %v\n%s", tag, err, out)
	}
	os.Stdout.Write(out)
}

type tagArgs struct {
	Message  string `json:"message"`
	Revision string `json:"revision"`
}

type tagReply struct {
	Object  string `json:"object"`
	Message string `json:"message"`
	Ref     string `json:"ref"`
}

type branchReply struct {
	Ref      string `json:"ref"`
	Revision string `json:"revision"`
}

func gerrit(method, url string, in, out any) {
	args := []string{"gob-curl", "-X", method}
	if in != nil {
		args = append(args, "-H", "Content-Type: application/json")
		js, err := json.Marshal(in)
		if err != nil {
			log.Fatalf("%s %s: %v", method, url, err)
		}
		args = append(args, "-d", string(js))
	}
	args = append(args, url)
	js := run(args...)
	if out != nil {
		println(string(js))
		_, js, _ = bytes.Cut(js, []byte("\n"))
		if err := json.Unmarshal(js, out); err != nil {
			log.Fatalf("%s %s: %v", method, url, err)
		}
	}
}

func gerrit1(method, url string, in, out any) {
	var body io.Reader
	if in != nil {
		js, err := json.Marshal(in)
		if err != nil {
			log.Fatalf("%s %s: %v", method, url, err)
		}
		body = bytes.NewReader(js)
	}
	req, err := http.NewRequest(method, url, body)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", method, url, err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("%s %s: %v", method, url, resp.Status)
	}
	if out != nil {
		js, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("%s %s: %v", method, url, resp.Status)
		}
		_, js, _ = bytes.Cut(js, []byte("\n"))
		if err := json.Unmarshal(js, out); err != nil {
			log.Fatalf("%s %s: %v", method, url, err)
		}
	}
	resp.Body.Close()
}

func runInErr(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = "/tmp/re2"
	var stdout, stderr bytes.Buffer
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return append(stderr.Bytes(), stdout.Bytes()...), err
	}
	return stdout.Bytes(), nil
}

func runErr(args ...string) ([]byte, error) {
	return runInErr(nil, args...)
}

func run(args ...string) []byte {
	return runIn(nil, args...)
}

func runIn(stdin []byte, args ...string) []byte {
	out, err := runInErr(stdin, args...)
	if err != nil {
		log.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}
