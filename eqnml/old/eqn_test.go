// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eqnml

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
	"rsc.io/diff"
)

func Test(t *testing.T) {
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".txt"), func(t *testing.T) {
			a, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}

			var ncase, npass int
			for i := 0; i+2 <= len(a.Files); i += 2 {
				ncase++
				eqn := a.Files[i]
				html := a.Files[i+1]
				name := strings.TrimSuffix(eqn.Name, ".eqn")
				if name != strings.TrimSuffix(html.Name, ".html") {
					t.Fatalf("mismatched file pair: %s and %s", eqn.Name, html.Name)
				}
				t.Run(name, func(t *testing.T) {
					h, err := ToHTML(string(eqn.Data))
					if err == nil {
						h += "\n"
					} else {
						h = "ERROR: " + err.Error()
						if !strings.HasSuffix(h, "\n") {
							h += "\n"
						}
					}
					if h != string(html.Data) {
						t.Fatalf("%s: wrong output: diff -want +have:\n%s", eqn.Name, diff.Format(h, string(html.Data)))
					}
					npass++
				})
			}
			t.Logf("%d/%d pass", npass, ncase)
		})
	}
}
