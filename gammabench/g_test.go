// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gammabench

import "testing"
import . "math"

// Global exported variables are used to store the
// return values of functions measured in the benchmarks.
// Storing the results in these variables prevents the compiler
// from completely optimizing the benchmarked functions away.
var (
	GlobalI int
	GlobalB bool
	GlobalF float64
)

func BenchmarkLgamma16(b *testing.B) {
	x := 0.0
	y := 0
	for i := 0; i < b.N; i++ {
		x, y = Lgamma16(2.5)
	}
	GlobalF = x
	GlobalI = y
}

func BenchmarkLgamma17(b *testing.B) {
	x := 0.0
	y := 0
	for i := 0; i < b.N; i++ {
		x, y = Lgamma17(2.5)
	}
	GlobalF = x
	GlobalI = y
}

func BenchmarkLgammaZZZ(b *testing.B) {
	x := 0.0
	y := 0
	for i := 0; i < b.N; i++ {
		x, y = LgammaZZZ(2.5)
	}
	GlobalF = x
	GlobalI = y
}

func TestLgamma(t *testing.T) {
	try := func(Lgamma func(float64) (float64, int)) func(t *testing.T) {
		return func(t *testing.T) {
			for i := 0; i < len(vf); i++ {
				if f, s := Lgamma(vf[i]); !close(lgamma[i].f, f) || lgamma[i].i != s {
					t.Errorf("Lgamma(%g) = %g, %d, want %g, %d", vf[i], f, s, lgamma[i].f, lgamma[i].i)
				}
			}
			for i := 0; i < len(vflgammaSC); i++ {
				if f, s := Lgamma(vflgammaSC[i]); !alike(lgammaSC[i].f, f) || lgammaSC[i].i != s {
					t.Errorf("Lgamma(%g) = %g, %d, want %g, %d", vflgammaSC[i], f, s, lgammaSC[i].f, lgammaSC[i].i)
				}
			}
		}
	}

	t.Run("Lgamma16", try(Lgamma16))
	t.Run("Lgamma17", try(Lgamma17))
	t.Run("LgammaZZZ", try(LgammaZZZ))
}

var vf = []float64{
	4.9790119248836735e+00,
	7.7388724745781045e+00,
	-2.7688005719200159e-01,
	-5.0106036182710749e+00,
	9.6362937071984173e+00,
	2.9263772392439646e+00,
	5.2290834314593066e+00,
	2.7279399104360102e+00,
	1.8253080916808550e+00,
	-8.6859247685756013e+00,
}

type fi struct {
	f float64
	i int
}

var lgamma = []fi{
	{3.146492141244545774319734e+00, 1},
	{8.003414490659126375852113e+00, 1},
	{1.517575735509779707488106e+00, -1},
	{-2.588480028182145853558748e-01, 1},
	{1.1989897050205555002007985e+01, 1},
	{6.262899811091257519386906e-01, 1},
	{3.5287924899091566764846037e+00, 1},
	{4.5725644770161182299423372e-01, 1},
	{-6.363667087767961257654854e-02, 1},
	{-1.077385130910300066425564e+01, -1},
}

var vflgammaSC = []float64{
	Inf(-1),
	-3,
	0,
	1,
	2,
	Inf(1),
	NaN(),
}
var lgammaSC = []fi{
	{Inf(-1), 1},
	{Inf(1), 1},
	{Inf(1), 1},
	{0, 1},
	{0, 1},
	{Inf(1), 1},
	{NaN(), 1},
}

func tolerance(a, b, e float64) bool {
	// Multiplying by e here can underflow denormal values to zero.
	// Check a==b so that at least if a and b are small and identical
	// we say they match.
	if a == b {
		return true
	}
	d := a - b
	if d < 0 {
		d = -d
	}

	// note: b is correct (expected) value, a is actual value.
	// make error tolerance a fraction of b, not a.
	if b != 0 {
		e = e * b
		if e < 0 {
			e = -e
		}
	}
	return d < e
}
func close(a, b float64) bool      { return tolerance(a, b, 1e-14) }
func veryclose(a, b float64) bool  { return tolerance(a, b, 4e-16) }
func soclose(a, b, e float64) bool { return tolerance(a, b, e) }
func alike(a, b float64) bool {
	switch {
	case IsNaN(a) && IsNaN(b):
		return true
	case a == b:
		return Signbit(a) == Signbit(b)
	}
	return false
}
