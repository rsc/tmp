// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build gccgo

package fieldtrack

import (
	"runtime"
	"sort"
	"sync"
)

// tracked holds the information gathered by the compiler and linker.
var tracked struct {
	once sync.Once
	m    map[string]bool
}

func initTracked() {
	tracked.m = make(map[string]bool)
	runtime.Fieldtrack(tracked.m)
}

// The sentinel type is a dummy type that we know should appear
// in the field tracking information, so that we can tell if field tracking
// was enabled during compilation.
type sentinel struct {
	F int `go:"track"`
}

// Whether field tracking was enabled in the compiler (doc comment in
// fieldtrack.go).

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
	tracked.once.Do(initTracked)
	return len(tracked.m) > 0
}

// List of all tracked fields (doc comment in fieldtrack.go).

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

	r := make([]string, 0, len(tracked.m))
	for k := range tracked.m {
		// Remove reference to our internal sentinel.
		if k != "rsc.io/tmp/fieldtrack.sentinel.F" {
			r = append(r, k)
		}
	}
	sort.Strings(r)
	trackedFields.list = r
}

// Ref returns the list of references (doc comment in fieldtrack.go).
// The gccgo implementation does not track references.

func Ref(field string) []string {
	if !Enabled() {
		panic("fieldtrack: compiler is not tracking fields")
	}

	if tracked.m[field] {
		return []string{"no.gccgo.refs"}
	}
	return nil
}
