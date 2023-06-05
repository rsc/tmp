// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var htmlTests = []struct {
	in  string
	out string
}{
	{`
		hello world
	`, `
		hello world
	`},
	{`
		hello <i>world</i>
	`, `
		hello _world_
	`},
	{`
		<b>hello</b> <i>world</i>
	`, `
		**hello** _world_
	`},
	{`
		<strong>hello</strong> <em>world</em>
	`, `
		**hello** _world_
	`},
	{`
		<strong>hello</strong> <dfn>world</dfn>
	`, `
		**hello** _world_
	`},
	{`
		<strong>hello</strong> <var>world</var>
	`, `
		**hello** _world_
	`},
	{`
		<code>hello</code>
	`, "`hello`\n",
	},
	{`
		<code id="x">hello</code>
	`, `
		<code id="x">hello</code>
	`},
	{`
		<code>hello <i>world</i></code>
	`, `
		<code>hello _world_</code>
	`,
	},
	{`
		<small>a</small>
		<span>b</span>
		<sup>c</sup>
		<sub>d</sub>
	`, `
		<small>a</small>
		<span>b</span>
		<sup>c</sup>
		<sub>d</sub>
	`,
	},
	{`
		<p><!-- hello --> world</p>
	`, `
		<!-- hello --> world
	`},
	{`
		<script>hello world</script>
	`, `
		<script>hello world</script>
	`},
	{`
		<script></script>
	`, `
		<script></script>
	`},
	{`
		<p>
		For the ARM 32-bit port, the assembler now supports the instructions
		<code><small>BFC</small></code>,
		<code><small>BFI</small></code>,
		and
		<code><small>XTAHU</small></code>.
		</p>
	`, `
		For the ARM 32-bit port, the assembler now supports the instructions
		<code><small>BFC</small></code>,
		<code><small>BFI</small></code>,
		and
		<code><small>XTAHU</small></code>.
	`},
	{`
		<p>
		  <strong>
		    Go 1.21 is not yet released. These are work-in-progress
		    release notes. Go 1.21 is expected to be released in August 2023.
		  </strong>
		</p>
	`, `
		**Go 1.21 is not yet released. These are work-in-progress
		release notes. Go 1.21 is expected to be released in August 2023.**
	`},
}

func TestHTML(t *testing.T) {
	for i, tt := range htmlTests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			out, err := html2md("#"+fmt.Sprint(i), stripTabs(tt.in))
			if err != nil {
				t.Fatal(err)
			}
			want := stripTabs(tt.out)
			if out != want {
				t.Fatalf("have:\n%s\nwant:\n%s\n", out, want)
			}
		})
	}
}

func TestDoc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	filepath.Walk("/Users/rsc/src/golang.org/x/website/_content", func(path string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		md, err := html2md(path, string(data))
		if err != nil {
			t.Errorf("%s: %v", path, err)
		}
		//println(md)
		_ = md
		return nil
	})
}
