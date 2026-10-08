// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eqnml

import (
	"bytes"
	"fmt"
	"testing"
)

var texParserTests = []struct {
	in  string
	out string
}{
	{`hello world`, `h e l l o w o r l d`},
	{`esc \backslash \ \{ \} \$ \& \# \_ \%`, `e s c \ ␣ { } $ & # _ %`},
	{`hi \bad escape`, `h i ERROR: input:1: undefined: \bad`},
	{`\def\foo{bar} hi \foo`, `h i b a r`},
	{`\def\smile☺ \smile world`, `☺ w o r l d`},
}

func TestTexParser(t *testing.T) {
	for i, tt := range texParserTests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			p := newTexParser()
			p.push(tokens(pos{"input", 1}, tt.in)...)
			at := pos{"input", 1}
			maxLen := len(tt.out)*2 + 10
			var buf bytes.Buffer
			func() {
				defer func() {
					if e := recover(); e != nil {
						if buf.Len() > 0 {
							buf.WriteString(" ")
						}
						fmt.Fprintf(&buf, "ERROR: %v", e)
					}
				}()

				for {
					t := p.next()
					if t.eof() {
						break
					}
					if buf.Len() > 0 {
						buf.WriteString(" ")
					}
					if t.pos != at {
						fmt.Fprintf(&buf, "@%s:%d ", t.pos.file, t.pos.line)
					}
					if t.special {
						buf.WriteString("!")
					}
					if t.s == " " {
						buf.WriteString("␣")
					} else {
						buf.WriteString(t.s)
					}
					if buf.Len() > maxLen {
						buf.WriteString(" ...")
						break
					}
				}
			}()
			out := buf.String()
			if out != tt.out {
				t.Errorf("input:\n%s\nhave: %s\nwant: %s", tt.in, out, tt.out)
			}
		})
	}
}
