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

	"rsc.io/tmp/ftoa/abseil"
	"rsc.io/tmp/ftoa/dblconv"
	"rsc.io/tmp/ftoa/dmg"
	"rsc.io/tmp/ftoa/fast_float"
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
	// http://swtch.com/dmg-fmt.pdf
	// Table 3 "typical cases"
	{1.23, 6},
	{1.23e+20, 6},
	{1.23e-20, 6},
	{1.23456789, 6},
	{1.23456589e+20, 6},
	{1.23456789e-20, 6},
	{1234565, 6},

	// Table 4 "hard cases"
	{1.234565, 6},
	{1.234565e+20, 6},
	{1.234565e-20, 6},

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
	{"ryu", ryu.Loop, 18},
	{"dmg1997", dmg.Loop1997, 18},
	{"dmg2016", dmg.Loop2016, 18},
	{"dmg2017", dmg.Loop2017, 18},
	{"dmg2025", dmg.Loop2025, 18},
	{"dblconv", dblconv.Loop, 3}, // dblconv rounds 0.5 up
	{"go124ryu", go124.LoopRyu, 18},
	{"gcvt", gcvtLoop, 17},
	{"cxx", cxxLoop, 17},
	{"AppendFloat", appendFloatLoop, 18},
	{"Appendf", appendfLoop, 18},
	{"go124unopt", go124.LoopUnopt, 18},
	{"snprintf", snprintfLoop, 17},
	{"ken", ken.Loop, 5},
	{"rsc", rsc.Loop, 13},
}

type formatSum struct {
	name string
	fn   func(int, []float64, int) int64
}

var formatSums = []formatSum{
	{"ftoa", ftoaLoopSum},
	{"ryu", ryu.LoopSum},
	{"dblconv", dblconv.LoopSum},
	{"go124ryu", go124.LoopSumRyu},
	{"go124unopt", go124.LoopSumUnopt},
	{"gcvt", gcvtLoopSum},
	{"cxx", cxxLoopSum},
	{"AppendFloat", appendFloatLoopSum},
	{"dmg1997", dmg.LoopSum1997},
	{"dmg2016", dmg.LoopSum2016},
	{"dmg2017", dmg.LoopSum2017},
	{"dmg2025", dmg.LoopSum2025},
}

var canadaFloats = parseCanada()

func parseCanada() []float64 {
	var fs []float64
	for line := range strings.Lines(canada) {
		f, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
		if err != nil {
			panic(err)
		}
		fs = append(fs, math.Abs(f))
	}
	return fs
}

func BenchmarkFormat(b *testing.B) {
	for _, in := range inputs {
		var buf [100]byte
		for _, impl := range alts {
			b.Run(fmt.Sprintf("f=%g/prec=%d/impl=%s", in.f, in.prec, impl.name), func(b *testing.B) {
				impl.fn(buf[:0], b.N, in.f, in.prec)
			})
		}
	}
}

func TestFormatCanada(t *testing.T) {
	for i := range canadaFloats {
		want := ftoaLoopSum(1, canadaFloats[i:i+1], 2)
		have := ryu.LoopSum(1, canadaFloats[i:i+1], 2)
		if have != want {
			println(canadaFloats[i], "HAVE", have, string(ftoaLoop(nil, 1, canadaFloats[i], 2)), "WANT", want, string(ryu.Loop(nil, 1, canadaFloats[i], 2)))
		}
	}
	want := []int64{5979925, 5976396, 5976389, 5976389}
	for _, impl := range formatSums {
		t.Run(impl.name, func(t *testing.T) {
			for i, prec := range []int{2, 5, 10, 17} {
				have := impl.fn(1, canadaFloats, prec)
				if have != want[i] {
					t.Errorf("formatSum(prec=%d) = %v, want %v", prec, have, want[i])
				}
			}
		})
	}
}

func BenchmarkFormatCanada(b *testing.B) {
	for _, prec := range []int{2, 5, 10, 17} {
		for _, impl := range formatSums {
			b.Run(fmt.Sprintf("prec=%d/impl=%s", prec, impl.name), func(b *testing.B) {
				impl.fn(b.N/len(canadaFloats), canadaFloats, prec)
			})
		}
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

func ftoaLoopSum(n int, fs []float64, prec int) int64 {
	var buf [100]byte
	var out int64
	for range n {
		total := int64(0)
		for _, f := range fs {
			efmt(buf[:], f, prec)
			total += int64(buf[0])
		}
		out = total
	}
	return out
}

func appendFloatLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	i := 0
	for range n {
		i = len(strconv.AppendFloat(buf[:0], f, 'e', prec-1, 64))
	}
	return append(dst, buf[:i]...)
}

func appendFloatLoopSum(n int, fs []float64, prec int) int64 {
	var buf [100]byte
	var out int64
	for range n {
		total := int64(0)
		for _, f := range fs {
			strconv.AppendFloat(buf[:0], f, 'e', prec-1, 64)
			total += int64(buf[0])
		}
		out = total
	}
	return out
}

func appendfLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	i := 0
	for range n {
		i = len(fmt.Appendf(buf[:0], "%.*e", prec-1, f))
	}
	return append(dst, buf[:i]...)
}

type parseAlt struct {
	name string
	fn   func(int, string) float64
}

var parseAlts = []parseAlt{
	{"ParseFloat", parseFloatLoop},
	{"fast_float", fast_float.LoopStrtod},
	{"abseil", abseil.LoopStrtod},
	{"strtod", strtodLoop},
	{"ken", ken.LoopStrtod},
	{"dmg1997", dmg.LoopStrtod1997},
	{"dmg2016", dmg.LoopStrtod2016},
	{"dmg2017", dmg.LoopStrtod2017},
	{"dmg2025", dmg.LoopStrtod2025},
}

var parseInputs = []string{
	"1.2345",
	"1.3553e142",
	"9.8765432101234567",
	"43.928328999999962",
	"66.294723999999917",
}

func TestAltParse(t *testing.T) {
	for _, impl := range parseAlts {
		t.Run(impl.name, func(t *testing.T) {
			for _, in := range parseInputs {
				want, err := strconv.ParseFloat(in, 64)
				if err != nil {
					t.Fatal(err)
				}
				have := impl.fn(1, in)
				if have != want {
					t.Errorf("strtod(%s) = %v, want %v", in, have, want)
				}
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	for _, in := range parseInputs {
		for _, impl := range parseAlts {
			b.Run(fmt.Sprintf("s=%s/impl=%s", in, impl.name), func(b *testing.B) {
				b.SetBytes(int64(len(in)))
				impl.fn(b.N, in)
			})
		}
	}
}

func parseFloatLoop(n int, s string) float64 {
	var f float64
	for range n {
		f, _ = strconv.ParseFloat(s, 64)
	}
	return f
}

type sum struct {
	name string
	fn   func(int, string) float64
}

var sums = []sum{
	{"ParseFloat", loopSumParseFloat},
	{"abseil", abseil.LoopSumStrtod},
	{"fast_float", fast_float.LoopSumStrtod},
	{"strtod", strtodLoopSum},
	{"ken", ken.LoopSumStrtod},
	{"dmg1997", dmg.LoopSumStrtod1997},
	{"dmg2016", dmg.LoopSumStrtod2016},
	{"dmg2017", dmg.LoopSumStrtod2017},
	{"dmg2025", dmg.LoopSumStrtod2025},
}

//go:embed testdata/canada.txt
var canada string

var canadaTotal = -1.265531108883936e+06

func TestCanada(t *testing.T) {
	for _, impl := range sums {
		t.Run(impl.name, func(t *testing.T) {
			f := impl.fn(1, canada)
			if f != canadaTotal {
				t.Fatalf("Sum = %v, want %v", f, canadaTotal)
			}
		})
	}
}

func BenchmarkParseCanada(b *testing.B) {
	for _, impl := range sums {
		b.Run(fmt.Sprintf("impl=%s", impl.name), func(b *testing.B) {
			b.SetBytes(int64(len(canada)))
			impl.fn(b.N, canada)
		})
	}
}

func loopSumParseFloat(n int, s string) float64 {
	var f float64
	for range n {
		start := 0
		total := 0.0
		for i, c := range []byte(s) {
			if c == '\n' {
				ff, _ := strconv.ParseFloat(s[start:i], 64)
				total += ff
				start = i + 1
			}
		}
		f = total
	}
	return f
}
