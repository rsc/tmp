// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ftoa

import (
	"bytes"
	_ "embed"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"rsc.io/tmp/ftoa/dblconv"
	"rsc.io/tmp/ftoa/dmg"
	"rsc.io/tmp/ftoa/go124"
	"rsc.io/tmp/ftoa/ken"
	"rsc.io/tmp/ftoa/rsc"
	"rsc.io/tmp/ftoa/ryu"
)

//go:embed test.ivy
var testIvy string

var ivyRE = regexp.MustCompile(`\(([0-9]+) ftoa ([^ ]+)\) is ([0-9]+) (-?[0-9]+)`)

func TestFtoa(t *testing.T) {
	testFtoa(t, 18, ftoa)
}

func testFtoa(t *testing.T, exact int, ftoa func(float64, int) (uint64, int)) {
	fail := 0
	for line := range strings.Lines(testIvy) {
		m := ivyRE.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("bad line: %s", line)
		}
		prec, _ := strconv.Atoi(m[1])
		if prec > exact {
			continue
		}
		f, _ := strconv.ParseFloat(m[2], 64)
		if exact != 18 && f < 0x1.0p-1022 {
			continue
		}
		want, _ := strconv.ParseUint(m[3], 10, 64)
		exp, _ := strconv.Atoi(m[4])
		dm, dp := ftoa(f, prec)
		if dm != want || dp != exp {
			t.Errorf("ftoa(%#x, %d) = %d, %d, want %d, %d", f, prec, dm, dp, want, exp)
			if fail++; fail >= 20 {
				t.Fatalf("too many failures")
			}
		}
	}
}

func TestAlt(t *testing.T) {
	for _, impl := range alts {
		t.Run(impl.name, func(t *testing.T) {
			testLoop(t, impl.exact, impl.fn)
		})
	}
}

func testLoop(t *testing.T, exact int, loop func([]byte, int, float64, int) []byte) {
	testFtoa(t, exact, func(f float64, prec int) (uint64, int) {
		var err error
		b := loop(nil, 1, f, prec)
		m, e, ok := bytes.Cut(b, []byte("e"))
		var mp []byte
		adj := 0
		for i, c := range m {
			if i == 0 && c == '0' {
				continue
			}
			if c == '.' {
				adj = i - (len(m) - 1)
			} else {
				mp = append(mp, c)
			}
		}
		mi, err := strconv.ParseUint(string(mp), 10, 64)
		if err != nil {
			t.Fatalf("malformed output: %s", b)
		}
		if len(mp) < prec {
			for range prec - len(mp) {
				mi *= 10
				adj--
			}
		}
		ei := int64(0)
		if ok {
			ei, err = strconv.ParseInt(string(e), 10, 32)
			if err != nil {
				t.Fatalf("malformed output: %s", b)
			}
		}
		return mi, int(ei) + adj
	})
}

var inputs = []struct {
	f    float64
	prec int
}{
	{1.0, 10},
	{1.234, 5},
	{0.0001234, 5},
	{math.Pi, 17},
	{math.Pi * 1e50, 17},
	{math.Pi * 1e100, 17},
	{math.Pi * 1e200, 17},
	{math.Pi * 1e300, 17},
	{math.Pi * 1e-50, 17},
	{math.Pi * 1e-100, 17},
	{math.Pi * 1e-200, 17},
	{math.Pi * 1e-300, 17},
}

type alt struct {
	name  string
	fn    func([]byte, int, float64, int) []byte
	exact int
}

var alts = []alt{
	{"ftoa", ftoaLoop, 18},
	{"AppendFloat", appendFloatLoop, 18},
	{"Appendf", appendfLoop, 18},
	{"go124ryu", go124.LoopRyu, 18},
	{"go124unopt", go124.LoopUnopt, 18},
	{"gcvt", gcvtLoop, 17},
	{"snprintf", snprintfLoop, 17},
	{"dblconv", dblconv.Loop, 3}, // dblconv rounds 0.5 up
	{"ryu", ryu.Loop, 18},
	{"dmg", dmg.Loop, 18},
	{"ken", ken.Loop, 5},
	{"rsc", rsc.Loop, 13},
}

func BenchmarkFormat(b *testing.B) {
	for _, in := range inputs {
		var buf [100]byte
		b.Run(fmt.Sprintf("f=%.*e", in.prec-1, in.f), func(b *testing.B) {
			for _, impl := range alts {
				b.Run("impl="+impl.name, func(b *testing.B) {
					impl.fn(buf[:0], b.N, in.f, in.prec)
				})
			}
		})
	}
}

func ftoaLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	i := 0
	for range n {
		i = len(efmt(buf[:], f, prec))
	}
	return append(dst, buf[:i]...)
}

func appendFloatLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	i := 0
	for range n {
		i = len(strconv.AppendFloat(buf[:0], f, 'e', prec-1, 64))
	}
	return append(dst, buf[:i]...)
}

func appendfLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	i := 0
	for range n {
		i = len(fmt.Appendf(buf[:0], "%.*e", prec-1, f))
	}
	return append(dst, buf[:i]...)
}
