// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.


// Randdist is a program to explore the distribution of rand.Float64.
// Run with:
//
//	go get rsc.io/devweb
//	go get -d rsc.io/tmp/randdist
//	devweb rsc.io/tmp/randdist
//	
// And then open $GOPATH/rsc.io/tmp/randdist/graph/x.html in a browser.
package main

import (
	"net/http"

	"rsc.io/devweb/slave"
	"rsc.io/tmp/randdist/graph"
)

func init() {
	http.HandleFunc("/graph", graph.Handler)
}

func main() {
	slave.Main()
}
