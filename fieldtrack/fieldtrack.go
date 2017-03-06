// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build gc

// Package fieldtrack provides access to information in the current binary
// that tracks use of struct fields.
// This information is computed at build time for struct fields marked
// with the `go:"track"` tag. See http://go/fieldtrack for background.
//
// To test run:
//
//     GOEXPERIMENT=fieldtrack ./make.bash
//     go test rsc.io/tmp/fieldtrack -ldflags=-k=rsc.io/tmp/fieldtrack.tracked
package fieldtrack

import (
	"sort"
	"strings"
	"sync"
)

// tracked holds the raw field tracking information supplied by the linker,
// provided the linker is invoked with -k rsc.io/tmp/fieldtrack.tracked.
//
// The raw tracking information is a sequence of lines, each terminated by
// a \n and describing a single tracked field referred to by the program.
// Each line is made up of one or more tab-separated fields.
// The first field is the name of the tracked field, fully qualified,
// as in "my/pkg.T.F". Subsequent fields give a shortest path of
// reverse references from that field to main.init or main.main,
// corresponding to one way in which the program might reach that field.
var tracked string

// The sentinel type is a dummy type that we know should appear
// in the field tracking information, so that we can tell if field tracking
// was enabled during compilation.
type sentinel struct {
	F int `go:"track"`
}

// Enabled reports whether field tracking was enabled in the compiler
// when compiling the current binary.
func Enabled() bool {
	// Code to make the compiler think we refer to sentinel.F.
	// Complex enough that it won't be optimized away.
	f := Enabled
	alwaysFalse := f == nil
	if alwaysFalse {
		var x sentinel
		println(x.F)
	}

	// If there's tracking information available, then field tracking is turned on.
	// If not, then sentinel.F is missing so it must be disabled.
	return tracked != ""
}

// Tracked returns a sorted list of all tracked fields used by the current binary.
// The form of each entry is "full/import/path.TypeName.FieldName".
func Tracked() []string {
	trackedFields.once.Do(initTrackedFields)
	return trackedFields.list
}

var trackedFields struct {
	once sync.Once
	list []string
}

func initTrackedFields() {
	if !Enabled() {
		panic("fieldtrack: compiler is not tracking fields")
	}

	x := strings.Split(tracked, "\n")

	// Cut path information, which starts at the first tab on each line.
	for i, line := range x {
		j := strings.Index(line, "\t")
		if j >= 0 {
			x[i] = line[:j]
		}
	}

	// Remove reference to our internal sentinel.F
	// and remove the "" Split found after the final \n.
	w := 0
	for _, line := range x {
		if line != "rsc.io/tmp/fieldtrack.sentinel.F" && line != "" {
			x[w] = line
			w++
		}
	}
	x = x[:w]

	sort.Strings(x)
	trackedFields.list = x
}

// Ref returns the list of references explaining why the given field name is used
// by the current binary. The field name should be in the form returned by Tracked.
// The references start at a global symbol and end with the field.
// If the field is not used by the current binary, Ref returns nil.
func Ref(field string) []string {
	refFields.once.Do(initRefFields)
	return refFields.m[field]
}

var refFields struct {
	once sync.Once
	m    map[string][]string
}

func initRefFields() {
	if !Enabled() {
		panic("fieldtrack: compiler is not tracking fields")
	}

	m := make(map[string][]string)
	for _, line := range strings.Split(tracked, "\n") {
		if line == "" {
			// empty string after final \n
			continue
		}

		f := strings.Split(line, "\t")

		field := f[0]

		// The path is presented from field back to the global;
		// we want a path from the global to the field, so reverse it.
		for i, j := 0, len(f)-1; i < j; i, j = i+1, j-1 {
			f[i], f[j] = f[j], f[i]
		}

		// Cut items from list before "main.main" or "main.init"
		// to skip the runtime details that get into main
		// (or else it starts with a global variable, which is fine).
		for i, x := range f {
			if x == "main.main" || x == "main.init" {
				f = f[i:]
				break
			}
		}

		m[field] = f
	}

	refFields.m = m
}
