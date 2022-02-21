// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Rmplay removes play.golang.org playground snippets.

Usage:

	rmplay https://go.dev/play/p/xxx...

Rmplay removes the snippets at each of the URLs named on the command line.
(We do this when people accidentally post sensitive material there and email
us at security@golang.org to take it down.)

Authentication

Rmplay expects to be able to use the Google Application Default Credentials
to access the Google Cloud Datastore. Typically this means one must run
“gcloud auth application-default login” before using rmplay.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/datastore"
)

const prefix = "https://go.dev/play/p/"

func usage() {
	fmt.Fprintf(os.Stderr, "usage: rmplay %sxxx...\n", prefix)
	os.Exit(2)
}

var (
	ctx    context.Context
	client *datastore.Client
)

func main() {
	log.SetPrefix("rmplay: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
	}

	ctx = context.Background()
	c, err := datastore.NewClient(ctx, "golang-org")
	if err != nil {
		log.Fatal(err)
	}
	client = c

	// Verify that we can communicate and authenticate with the datastore service.
	t, err := client.NewTransaction(ctx)
	if err != nil {
		log.Fatalf("cannot connect: %v", err)
	}
	if err := t.Rollback(); err != nil {
		log.Fatalf("cannot connect: %v", err)
	}

	for _, url := range flag.Args() {
		if !strings.HasPrefix(url, prefix) {
			log.Print("invalid URL: %s", url)
			continue
		}
		rmplay(strings.TrimPrefix(url, prefix))
	}
}

func rmplay(id string) {
	key := &datastore.Key{Kind: "Snippet", Name: id}
	err := client.Delete(ctx, key)
	if err != nil {
		log.Printf("%s: %v", id, err)
		return
	}
	log.Printf("%s: deleted", id)
}
