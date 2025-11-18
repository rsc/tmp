// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ftoa

import (
	_ "embed"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"rsc.io/tmp/ftoa/dblconv"
	"rsc.io/tmp/ftoa/ryu"
)

//go:embed test.ivy
var testIvy string

var ivyRE = regexp.MustCompile(`\(([0-9]+) ftoa ([^ ]+)\) is ([0-9]+) (-?[0-9]+)`)

func TestFtoa(t *testing.T) {
	testFtoa(t, ftoa)
}

func testFtoa(t *testing.T, ftoa func(float64, int) (uint64, int)) {
	for line := range strings.Lines(testIvy) {
		m := ivyRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("bad line: %s", line)
		}
		prec, _ := strconv.Atoi(m[1])
		if prec > 18 {
			continue
		}
		f, _ := strconv.ParseFloat(m[2], 64)
		want, _ := strconv.ParseUint(m[3], 10, 64)
		exp, _ := strconv.Atoi(m[4])
		dm, dp := ftoa(f, prec)
		if dm != want || dp != exp {
			t.Errorf("ftoa(%#x, %d) = %d, %d, want %d, %d", f, prec, dm, dp, want, exp)
		}
	}
}

var inputs = []struct {
	f    float64
	prec int
}{
	{1.0, 10},
	{1.234, 5},
	{0.0001234, 5},
	{math.Pi * 1e200, 17},
}

// TODO test the implementations.
// TODO add ryu

var impls = []struct {
	name string
	fn   func(*testing.B, float64, int)
}{
	{"ftoa", benchEfmt},
	{"snprintf", benchSnprintf},
	{"AppendFloat", benchAppendFloat},
	{"Appendf", benchAppendf},
	{"dblconv", benchDblconv},
	{"ryu", benchRyu},
}

func BenchmarkFormat(b *testing.B) {
	for _, impl := range impls {
		b.Run("impl="+impl.name, func(b *testing.B) {
			for _, in := range inputs {
				b.Run(fmt.Sprintf("f=%.*e", in.prec-1, in.f), func(b *testing.B) {
					impl.fn(b, in.f, in.prec)
				})
			}
		})
	}
}

func benchDblconv(b *testing.B, f float64, prec int) {
	dblconv.LoopEfmt(b.N, f, prec)
}

func benchRyu(b *testing.B, f float64, prec int) {
	ryu.LoopEfmt(b.N, f, prec)
}

func benchEfmt(b *testing.B, f float64, prec int) {
	var dst [100]byte
	for b.Loop() {
		efmt(dst[:], f, prec-1)
	}
}

func benchAppendFloat(b *testing.B, f float64, prec int) {
	var dst [100]byte
	for b.Loop() {
		strconv.AppendFloat(dst[:0], f, 'e', prec-1, 64)
	}
}

func benchAppendf(b *testing.B, f float64, prec int) {
	var dst [100]byte
	for b.Loop() {
		fmt.Appendf(dst[:0], "%.*e", prec-1, f)
	}
}

func benchSnprintf(b *testing.B, f float64, prec int) {
	if loopSnprintf == nil {
		b.Fatalf("snprintf not available")
	}
	loopSnprintf(b.N, f, prec-1)
}

func BenchmarkSnprintf0(b *testing.B) {
	if loopSnprintd == nil {
		b.Fatalf("snprintf not available")
	}
	loopSnprintd(b.N, 12345678901234567)
}
