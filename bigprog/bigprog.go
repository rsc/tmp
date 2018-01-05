// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	sizes := []int{1000, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 256000}
	cmds := [][]string{
		{"go", "tool", "compile"},
		{"gotype"},
		{"go", "vet"},
		{"gofmt"},
	}
	big := strings.Repeat("1", 1000)
	for _, cmd := range cmds {
		if len(os.Args) > 1 && !strings.Contains(strings.Join(cmd, " "), os.Args[1]) {
			continue
		}
		fmt.Printf("### %s\n\n", strings.Join(cmd, " "))
		lastStrbig := time.Duration(0)
		for _, size := range sizes {
			fmt.Printf("    %10d  int ", size)
			fmt.Printf("%s", runProg(cmd, "package p\nvar x = `1`"+strings.Repeat("+ `1`", size)+"\n"))
			fmt.Printf("  strbal ")
			fmt.Printf("%s", runProg(cmd, "package p\nvar x = "+balance(size, "1")+"\n"))
			fmt.Printf("  strbigbal ")
			fmt.Printf("%s", runProg(cmd, "package p\nvar x = "+balance(size, big)+"\n"))
			fmt.Printf("  str ")
			fmt.Printf("%s", runProg(cmd, "package p\nvar x = 1"+strings.Repeat("+ 1", size)+"\n"))
			if lastStrbig < 100*time.Second {
				fmt.Printf("  strbig  ")
				t := time.Now()
				result := runProg(cmd, "package p\nvar x = `1`"+strings.Repeat("+ `"+big+"`", size)+"\n")
				fmt.Printf("%s", result)
				lastStrbig = time.Since(t)
				if strings.Contains(result, "!") {
					lastStrbig = 1000 * time.Second
				}
			}
			fmt.Printf("\n")
		}
		fmt.Printf("\n")
	}
}

func balance(n int, s string) string {
	var buf bytes.Buffer
	balanceN(&buf, n+1, s)
	return buf.String()
}

func balanceN(buf *bytes.Buffer, n int, s string) {
	if n >= 2 {
		buf.WriteString("(")
		balanceN(buf, n/2, s)
		buf.WriteString(" + ")
		balanceN(buf, n-n/2, s)
		buf.WriteString(")")
		return
	}
	buf.WriteString("`")
	buf.WriteString(s)
	buf.WriteString("`")
}

func runProg(cmd []string, prog string) string {
	err := ioutil.WriteFile("/tmp/bigprog_tmp_.go", []byte(prog), 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove("/tmp/bigprog_tmp_.go")
	start := time.Now()
	out, err := exec.Command(cmd[0], append(cmd[1:], "/tmp/bigprog_tmp_.go")...).CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("runtime: goroutine stack exceeds")) {
			return "  stack!"
		}
		if strings.Contains(err.Error(), "signal: killed") || bytes.Contains(out, []byte("signal: killed")) {
			return "sigkill!"
		}
		if bytes.Contains(out, []byte("out of memory")) {
			if bytes.Contains(out, []byte("concatstring")) {
				return "    mem!"
			}
			if bytes.Contains(out, []byte("morestack")) {
				return "  stack!"
			}
		}
		log.Printf("exec: %s\n%s", err, out)
		return "  crash!"
	}
	return fmt.Sprintf("%7.3fs", time.Since(start).Seconds())
}
