// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Goshort manipulates golang.org/s short links.

Usage:

	goshort name [url]

Goshort prints or optionally sets the URL that golang.org/s/name redirects to.

Authentication

Goshort expects to be able to use the Google Application Default Credentials
to access the Google Cloud Datastore. Typically this means one must run
“gcloud auth application-default login” before using goshort.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/datastore"
)

const prefix = "https://golang.org/s/"

func usage() {
	fmt.Fprintf(os.Stderr, "usage: goshort name [url]\n")
	os.Exit(2)
}

var (
	ctx    context.Context
	client *datastore.Client
)

type Link struct {
	Key, Target string
}

func main() {
	log.SetPrefix("goshort: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 || len(args) > 2 {
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

	var link Link
	id := args[0]
	key := &datastore.Key{Kind: "Link", Name: id}
	err = client.Get(ctx, key, &link)
	if len(args) == 1 {
		if err != nil {
			log.Printf("%s: %v", id, err)
			return
		}
		fmt.Printf("%s\n", link.Target)
		return
	}
	if err == nil {
		log.Printf("old: golang.org/s/%s -> %s", link.Key, link.Target)
	}

	link.Key = id
	link.Target = args[1]
	_, err = client.Put(ctx, key, &link)
	if err != nil {
		log.Printf("%s: %v", id, err)
		return
	}
	log.Printf("new: golang.org/s/%s -> %s\n", link.Key, link.Target)
}
