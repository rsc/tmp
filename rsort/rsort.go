// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rsort

import (
	"fmt"
)

func isort(x []string, offset int) {
	if len(x) <= 1 {
		return
	}
	for i := range x {
		for j := i; j > 0 && x[j-1][offset:] > x[j][offset:]; j-- {
			x[j], x[j-1] = x[j-1], x[j]
		}
	}
}

const debug = false

func sortWithTmp(x []string) {
	sortWithTmp1(x, make([]string, len(x)), 0)
}

func sortWithTmp1(x, tmp []string, offset int) {
Loop:
	// divert
	if len(x) < 16 {
		isort(x, offset)
		return
	}

	// tally
	var counts, end [257]int
	cmin := 256
	cmax := 1
	for _, s := range x {
		c := 0
		if offset < len(s) {
			c = int(s[offset]) + 1
		}
		counts[c]++
		if counts[c] == 1 && c > 0 {
			cmin = min(cmin, c)
			cmax = max(cmax, c)
		}
	}

	// find places
	used := counts[0]
	end[0] = used
	maxc := 0
	maxcn := 0
	for c := cmin; c <= cmax; c++ {
		n := counts[c]
		if n == 0 {
			continue
		}
		used += counts[c]
		end[c] = used
		if n > maxcn {
			maxc, maxcn = c, n
		}
	}

	if debug {
		fmt.Println("x", offset, x)
		fmt.Println(counts)
		fmt.Println(end)
	}

	// move to temp
	copy(tmp, x)

	// move to home
	for i := len(x) - 1; i >= 0; i-- {
		s := tmp[i]
		c := 0
		if offset < len(s) {
			c = int(s[offset]) + 1
		}
		//		println(c)
		end[c]--
		x[end[c]] = s
	}

	if debug {
		fmt.Println("moved:", x)
	}

	// recursively sort sections, saving largest for “tail call” goto Loop.
	// Handling the largest in this stack frame guarantees that any
	// recursive call must handle ≤ len(x)/2 elements, guaranteeing
	// a logarithmic number of recursions.
	used = counts[0]
	var last []string
	for c := cmin; c <= cmax; c++ {
		n := counts[c]
		if c > 0 && n > 1 {
			if c == maxc {
				last = x[used : used+n]
			} else {
				sortWithTmp1(x[used:used+n], tmp, offset+1)
			}
		}
		used += n
	}
	if last != nil {
		x = last
		offset++
		goto Loop
	}
}

func sortInPlace(x []string) {
	sortInPlace1(x, 0)
}

func sortInPlace1(x []string, offset int) {
	const cut = 16
Loop:
	if debug {
		fmt.Println("inplace", len(x), offset)
	}

	// divert
	if len(x) < cut {
		isort(x, offset)
		return
	}

	// tally
	var counts [256]int
	cmin := 255
	cmax := 0
	z := 0
	for i, s := range x {
		if offset >= 0 && offset < len(s) {
			c := s[offset]
			cmin = min(cmin, int(c))
			cmax = max(cmax, int(c))
			counts[c]++
		} else {
			x[i] = x[z]
			x[z] = s
			z++
		}
	}
	if z > 0 {
		x = x[z:]
		if len(x) < cut {
			isort(x, offset)
			return
		}
	}

	// find places
	used := 0
	maxc := 0
	maxcn := 0
	lastc := 0
	var end [256]int
	for c := cmin; c <= cmax; c++ {
		n := counts[c]
		if n == 0 {
			continue
		}
		used += n
		end[c] = used
		if n > maxcn {
			maxc, maxcn = c, n
		}
		lastc = c
	}

	if debug {
		fmt.Println("x", offset, x)
		fmt.Println(counts)
		fmt.Println(end)
	}

	// permute home
	n := len(x) - counts[lastc]
	if debug {
		println("permute", n, len(x), maxcn, maxc)
	}
	for i := 0; i < n; {
		if debug {
			println("step", i)
		}
		s := x[i]
		var c byte
		for {
			c = s[offset]
			e := end[c] - 1
			end[c] = e
			if debug {
				println("s", s, "i", i, "c", c, "e", e)
			}
			if e <= i {
				// Note: e < i means we are looking at the first string
				// in a run that is entirely fixed already; breaking the loop
				// will skip out of it.
				break
			}
			s, x[e] = x[e], s
		}
		x[i] = s
		if debug {
			println("done", "i", i, "c", c)
		}
		i += counts[c]
	}

	if debug {
		fmt.Println("moved:", x)
	}

	// recursively sort sections
	// save biggest for iteration
	used = 0
	var last []string
	for c := cmin; c <= cmax; c++ {
		n := counts[c&0xFF]
		if c > 0 && n > 1 {
			sub := x[used : used+n]
			if c == maxc {
				last = sub
			} else {
				sortInPlace1(sub, offset+1)
			}
		}
		used += n
	}
	if last != nil {
		x = last
		offset++
		goto Loop
	}
}

func sortInPlaceJump(x []string) {
	sortInPlaceJump1(x, 0)
}

func sortInPlaceJump1(x []string, offset int) {
	const cut = 16
Loop:
	if debug {
		fmt.Println("inplacejump", len(x), offset)
	}
	// divert
	if len(x) < cut {
		isort(x, offset)
		return
	}

	if true {
	Offset:
		// See if x is entirely strings with the same next character.
		// Find first non-short string in x.
		// Skip over strings with len(x[i]) == offset at the start of x.
		x0 := x[0]
		if len(x0) == offset {
			i := 0
			for len(x0) == offset {
				i++
				if i == len(x) {
					return
				}
				x0 = x[i]
			}
			// Cut short strings from start of x; they're already sorted.
			x = x[i:]
			if len(x) < cut {
				isort(x, offset)
				return
			}
		}

		// Find last non-short string in x.
		j := len(x) - 1
		for j > 0 && len(x[j]) == offset {
			j--
		}
		xn := x[j]

		// If the first and last non-short string have the same char at offset,
		// check whether they all do. In fact, check whether they all
		// share the same next many chars at offset, up to a constant limit
		// to preserve asymptotics.
		if x0[offset] == xn[offset] {
			const maxPre = 32
			z := 0
			pre := len(x0) - offset
			if pre > maxPre {
				pre = maxPre
			}
			pre = 1
			c := x0[offset]
			for i, s := range x {
				if len(s) == offset {
					// Swap short string down to start of x.
					t := x[z]
					x[z] = s
					x[i] = t
					z++
					continue
				} else if s[offset] != c {
					pre = 0
					break
				} else if pre > 1 {
					i := 0
					for i < pre && offset+i < len(s) && x0[offset+i] == s[offset+i] {
						i++
					}
					pre = i
					if pre == 0 {
						break
					}
				}
			}
			offset += pre
			if z > 0 {
				// Cut short strings from start of x.
				x = x[z:]
				if len(x) < cut {
					isort(x, offset)
					return
				}
			}
			if pre == maxPre {
				goto Offset
			}
		}
	}

	// tally
	var counts [256]int
	cmin := 255
	cmax := 0
	z := 0
	for i, s := range x {
		if offset >= 0 && offset < len(s) {
			c := s[offset]
			cmin = min(cmin, int(c))
			cmax = max(cmax, int(c))
			counts[c]++
		} else {
			x[i] = x[z]
			x[z] = s
			z++
		}
	}
	if z > 0 {
		x = x[z:]
		if len(x) < cut {
			isort(x, offset)
			return
		}
	}

	// find places
	used := 0
	maxc := 0
	maxcn := 0
	lastc := 0
	var end [256]int
	for c := cmin; c <= cmax; c++ {
		n := counts[c]
		if n == 0 {
			continue
		}
		used += n
		end[c] = used
		if n > maxcn {
			maxc, maxcn = c, n
		}
		lastc = c
	}

	if debug {
		fmt.Println("x", offset, x)
		fmt.Println(counts)
		fmt.Println(end)
	}

	// permute home
	n := len(x) - counts[lastc]
	if debug {
		println("permute", n, len(x), maxcn, maxc)
	}
	for i := 0; i < n; {
		if debug {
			println("step", i)
		}
		s := x[i]
		var c byte
		for {
			c = s[offset]
			e := end[c] - 1
			end[c] = e
			if debug {
				println("s", s, "i", i, "c", c, "e", e)
			}
			if e <= i {
				// Note: e < i means we are looking at the first string
				// in a run that is entirely fixed already; breaking the loop
				// will skip out of it.
				break
			}
			s, x[e] = x[e], s
		}
		x[i] = s
		if debug {
			println("done", "i", i, "c", c)
		}
		i += counts[c]
	}

	if debug {
		fmt.Println("moved:", x)
	}

	// recursively sort sections
	// save biggest for iteration
	used = 0
	var last []string
	for c := cmin; c <= cmax; c++ {
		n := counts[c&0xFF]
		if c > 0 && n > 1 {
			sub := x[used : used+n]
			if c == maxc {
				last = sub
			} else {
				sortInPlace1(sub, offset+1)
			}
		}
		used += n
	}
	if last != nil {
		x = last
		offset++
		goto Loop
	}
}
