// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
	"testing"
)

var printTests = []struct {
	in  block
	out string
}{
	{
		nil,
		``,
	},
	{
		para{
			text(" hello world  "),
		},
		`
		hello world
		`,
	},
	{
		para{
			text("hello\n\nworld"),
		},
		`
		hello
		world
		`,
	},
	{
		para{
			text("hello\n\nworld"),
			text("asdf"),
			text("asdf"),
		},
		`
		hello
		worldasdfasdf
		`,
	},
	{
		para{
			text(" a "),
			text(" b "),
			text(" c "),
		},
		`
		a b c
		`,
	},
	{
		para{
			text(" a "),
			text("\n"),
			text(" b "),
		},
		`
		a
		b
		`,
	},
	{
		para{
			text(" a "),
			emph{text(" b ")},
			text(" c "),
		},
		`
		a _b_ c
		`,
	},
	{
		para{
			text(" a "),
			emph{text("b")},
			text(" c "),
		},
		`
		a _b_ c
		`,
	},
	{
		para{
			text(" a "),
			strong{text(" b ")},
			text(" c "),
		},
		`
		a **b** c
		`,
	},
	{
		para{
			text(" a "),
			strong{text("b")},
			text(" c "),
		},
		`
		a **b** c
		`,
	},
	{
		para{
			text(" a "),
			link{"URL", inlines{text(" b ")}},
			text(" c "),
		},
		`
		a [ b ](URL) c
		`,
	},
	{
		para{
			text(" a "),
			link{"https://go.dev", inlines{text("https://go.dev")}},
			text(" c "),
		},
		`
		a <https://go.dev> c
		`,
	},
	{
		para{
			text(" a "),
			code("hello"),
			text(" c "),
		},
		"a `hello` c\n",
	},
	{
		para{
			code("don`t worry"),
		},
		"``don`t worry``\n",
	},
	{
		para{
			code(" spaces "),
		},
		"`  spaces  `\n",
	},
	{
		para{
			code("spaces "),
		},
		"` spaces  `\n",
	},
	{
		para{
			code(" spaces"),
		},
		"`  spaces `\n",
	},
	{
		para{
			code("`quotes"),
		},
		"`` `quotes ``\n",
	},
	{
		para{
			code("quotes`"),
		},
		"`` quotes` ``\n",
	},
	{
		para{
			code("```meta``"),
		},
		"```` ```meta`` ````\n",
	},

	{
		para{
			text("hello\n\nworld"),
			text("asdf"),
			hardBreak{},
			text("asdf"),
		},
		`
		hello
		worldasdf \
		asdf
		`,
	},
	{
		heading{1, "", inlines{text("heading")}},
		"# heading\n",
	},
	{
		heading{3, "id", inlines{text("heading")}},
		"### heading {#id}\n",
	},
	{
		quote{
			blocks{
				para{text("a")},
				para{text("b")},
				para{text("c")},
			},
		},
		`
		  > a
		  >
		  > b
		  >
		  > c
		`,
	},
	{
		pre("\n\nhello\n\nworld\ntest\n\n"),
		`
			hello

			world
			test
		`,
	},
	{
		list{items: []blocks{
			{para{text("a")}},
			{para{text("b")}},
			{para{text("c")}},
		}},
		`
		  - a
		  - b
		  - c
		`,
	},
	{
		list{items: []blocks{
			{para{text("\na")}},
			{para{text("b")}},
			{para{text("c")}},
		}},
		`
		  - a
		  - b
		  - c
		`,
	},
	{
		list{items: []blocks{
			{para{text("a")}, para{text("z")}},
			{para{text("b")}},
			{para{text("c")}},
		}},
		`
		  - a

		    z

		  - b

		  - c
		`,
	},
	{
		list{num: 2, items: []blocks{
			{para{text("a")}},
			{para{text("b")}},
			{para{text("c")}},
		}},
		`
		 2. a
		 3. b
		 4. c
		`,
	},
	{
		defns{
			{inlines{text("a")}, blocks{para{text("alpha")}}},
			{inlines{text("bcd")}, blocks{para{text("beta")}, para{text("charlie")}, para{text("delta")}}},
		},
		`
		a
		:   alpha

		bcd
		:   beta

		    charlie

		    delta
		`,
	},
}

func TestPrint(t *testing.T) {
	for i, tt := range printTests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			out := mdprint(tt.in)
			want := stripTabs(tt.out)
			if out != want {
				t.Fatalf("have:\n%s\nwant:\n%s\n", out, want)
			}
		})
	}
}

func stripTabs(s string) string {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) == 1 {
		return lines[0]
	}
	if lines[0] == "\n" {
		lines = lines[1:]
		for i, l := range lines {
			if l == "\t" {
				l = ""
			}
			lines[i] = strings.TrimPrefix(l, "\t\t")
		}
	}
	return strings.Join(lines, "")
}
