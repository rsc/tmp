// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rsort

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

var algs = []struct {
	name string
	sort func([]string)
}{
	{"sortWithTmp", sortWithTmp},
	{"sortInPlace", sortInPlace},
	{"sortInPlaceJump", sortInPlaceJump},
	{"sort.Strings", sort.Strings},
	{"slices.Sort", slices.Sort[[]string]},
}

func BenchmarkText(b *testing.B) {
	files, err := filepath.Glob("testdata/*.txt.gz")
	if err != nil {
		b.Fatal(err)
	}
	if len(files) == 0 {
		b.Fatal("no testdata/*.txt.gz")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".txt.gz")
		b.Run("input="+name, func(b *testing.B) {
			lines, perm := readTxtGz(b, file)
			d := sortDepth(lines)
			for _, alg := range algs {
				b.Run("alg="+alg.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ReportMetric(d, "depth/op")
					for range b.N {
						copy(lines, perm)
						alg.sort(lines)
					}
				})
			}
		})
		b.Run("input="+name+".sortish", func(b *testing.B) {
			lines, perm := readTxtGz(b, file)
			sort.Strings(perm)
			r := rand.New(rand.NewPCG(0, 0))
			for range 10 {
				i, j := r.IntN(len(perm)), r.IntN(len(perm))
				perm[i], perm[j] = perm[j], perm[i]
			}
			d := sortDepth(lines)
			for _, alg := range algs {
				b.Run("alg="+alg.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ReportMetric(d, "depth/op")
					for range b.N {
						copy(lines, perm)
						alg.sort(lines)
					}
				})
			}
		})
	}
}

func readTxtGz(t testing.TB, file string) (lines, perm []string) {
	data, _ := os.ReadFile(file)
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(string(data), "\n")
	lines = lines[:len(lines)-1] // remove trailing empty string
	sort.Strings(lines)

	perm = make([]string, len(lines))
	for j, i := range rand.New(rand.NewPCG(0, 0)).Perm(len(lines)) {
		perm[i] = lines[j]
	}
	return
}

func BenchmarkRandom(b *testing.B) {
	for _, n := range []int{1 << 8, 1 << 12, 1 << 16, 1 << 20, 1 << 24} {
		b.Run(fmt.Sprint("n=", n), func(b *testing.B) {
			lines := make([]string, n)
			for _, s := range []int{8, 16, 32, 64, 128, 256, 512, 1024} {
				b.Run(fmt.Sprint("len=", s), func(b *testing.B) {
					for _, e := range []float64{8, 7, 4, 2, 1, 0.5, 0.25, 0.125, 0.0625} {
						var ename string
						if e >= 1 {
							ename = fmt.Sprint(e)
						} else {
							ename = fmt.Sprint("1_", 1/e)
						}
						b.Run("e="+ename, func(b *testing.B) {
							mixed := make([]string, n)
							r := rand.NewPCG(0, 0)
							for i := range mixed {
								mixed[i] = randString(r, s, int(e*float64(s)))
							}
							copy(lines, mixed)
							sort.Strings(lines)
							d := sortDepth(lines)
							for _, alg := range algs {
								b.Run("alg="+alg.name, func(b *testing.B) {
									b.ReportAllocs()
									b.ReportMetric(d, "depth/op")
									for range b.N {
										copy(lines, mixed)
										alg.sort(lines)
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

func sortDepth(lines []string) float64 {
	d := 0.0
	for i := 0; i+1 < len(lines); i++ {
		x, y := lines[i], lines[i+1]
		for j := 0; ; j++ {
			if j >= len(x) || j >= len(y) || x[j] != y[j] {
				d += float64(j)
				break
			}
		}
	}
	return d / float64(len(lines)-1)
}

func randString(r *rand.PCG, n int, bits int) string {
	s := make([]byte, n)
	used := 0
	for i := range n {
		b := min(i*bits/n-used, 8)
		used += b
		s[i] = ' ' + byte(r.Uint64()&(1<<b-1))
	}
	return string(s)
}
