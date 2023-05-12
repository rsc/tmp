// Newcmd does TODO
//
// Usage:
//
//	newcmd TODO
//
// Newcmd more explanation here TODO.
//
// TODO Delete the following:
//
// This package is not meant to run directly.
// Instead it is meant to be used with rsc.io/tmp/gonew, as in:
//
//	gonew rsc.io/tmp/newcmd myprog
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: newcmd TODO\n")
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("newcmd: ")
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 0 {
		usage()
	}

	// TODO use args and do stuff
}
