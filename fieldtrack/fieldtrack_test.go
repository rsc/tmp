// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fieldtrack_test

import (
	"reflect"
	"runtime"
	"testing"

	"rsc.io/tmp/fieldtrack"
)

// Field tracking test case.
// F, G, G2, H, H2, I, and I2 are used.
// G2, H2, and I2 are not tracked and should not appear in the list.
// J is not used.
type T struct {
	F  int `go:"track"`
	G  int `go:"track"`
	G2 int
	H  int `go:"track"`
	H2 int
	I  int `go:"track"`
	I2 int
	J  int `go:"track"`
}

//go:nointerface

func (t *T) JMethod() int {
	return t.J
}

var b bool

func noinline() {
	for b {
	}
}

func F(t *T) int {
	noinline()
	return t.F + G(t) + H(t)
}

func G(t *T) int {
	noinline()
	return t.G + t.G2
}

func H(t *T) int {
	noinline()
	return t.H + t.H2 + I(t)
}

func I(t *T) int {
	noinline()
	return t.I + t.I2 + G(t)
}

func J(t *T) int {
	noinline()
	return t.J + G(t)
}

func TestFieldtrack(t *testing.T) {
	if !fieldtrack.Enabled() {
		t.Fatalf("fieldtrack.Enabled() = false")
	}

	F(new(T))

	const pkg = "rsc.io/tmp/fieldtrack_test"

	fields := fieldtrack.Tracked()

	expect := []string{
		pkg + ".T.F",
		pkg + ".T.G",
		pkg + ".T.H",
		pkg + ".T.I",
		// but not J
	}

	if !reflect.DeepEqual(fields, expect) {
		t.Errorf("Tracked()=%v, want %v", fields, expect)
	}

	// gccgo does not track references.
	if runtime.Compiler == "gccgo" {
		t.Logf("skipping Ref test on gccgo")
		return
	}

	paths := map[string][]string{
		pkg + ".T.F": {
			pkg + ".TestFieldtrack",
			pkg + ".F",
			pkg + ".T.F",
		},
		pkg + ".T.G": {
			pkg + ".TestFieldtrack",
			pkg + ".F",
			pkg + ".G",
			pkg + ".T.G",
		},
		pkg + ".T.H": {
			pkg + ".TestFieldtrack",
			pkg + ".F",
			pkg + ".H",
			pkg + ".T.H",
		},
		pkg + ".T.I": {
			pkg + ".TestFieldtrack",
			pkg + ".F",
			pkg + ".H",
			pkg + ".I",
			pkg + ".T.I",
		},
	}

	for _, f := range expect {
		path := fieldtrack.Ref(f)
		// drop everything before TestFieldtrack, which might differ
		// depending on exactly how the list of tests gets initialized.
		for i, x := range path {
			if x == pkg+".TestFieldtrack" {
				path = path[i:]
				break
			}
		}
		if !reflect.DeepEqual(path, paths[f]) {
			t.Errorf("Ref(%v) = %v, want %v", f, path, paths[f])
		}
	}
}
