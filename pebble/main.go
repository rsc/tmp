// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pebble is an interactive editor for [github.com/cockroachdb/pebble] databases.
//
// Usage:
//
//	pebble [-c] database
//
// The -c flag indicates that pebble should create a new database
// if it does not exist already. Otherwise, naming a non-existent
// database is an error.
//
// At the > prompt, the following commands are supported:
//
//	get(key [, end])
//	hex(key [, end])
//	list(start, end)
//	set(key, value)
//	delete(key [, end])
//	mvprefix(old, new)
//
// Get prints the value associated with the given key.
// If the end argument is given, get prints all key, value pairs
// with key k satisfying key ≤ k ≤ end.
//
// Hex is similar to get but prints hexadecimal dumps of
// the values instead of using value syntax.
//
// List lists all known keys k such that start ≤ k < end,
// but not their values.
//
// Set sets the value associated with the given key.
//
// Delete deletes the entry with the given key,
// printing an error if no such entry exists.
// If the end argument is given, delete deletes all entries
// with key k satisfying key ≤ k ≤ end.
//
// Mvprefix replaces every database entry with a key starting with old
// by an entry with a key starting with new instead (s/old/new/).
//
// Each of the key, value, start, and end arguments can be a
// Go quoted string or else a Go expression o(list) denoting an
// an [ordered code] value encoding the values in the argument list.
// The values in the list can be:
//
//   - a string value: a Go quoted string
//   - an ordered.Infinity value: the name Inf.
//   - an integer value: a possibly-signed integer literal
//   - a float64 value: a floating-point literal number (including a '.', 'e', or ,'p')
//   - a float64 value: float64(f) where f is an integer or floating-point literal
//     or NaN, Inf, +Inf, or -Inf
//   - a float32 value: float32(f) where f is an integer or floating-point literal
//     or NaN, Inf, +Inf, or -Inf
//   - rev(x) where x is one of the preceding choices, for a reverse-ordered value
//
// Note that Inf is an ordered infinity, while float64(Inf) is a floating-point infinity.
//
// The command output uses the same syntax to print keys and values.
//
// [ordered code]: https://pkg.go.dev/rsc.io/ordered
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"math"
	"os"
	"strconv"

	"github.com/cockroachdb/pebble"
	"rsc.io/ordered"
)

var createDB = flag.Bool("c", false, "create database")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: pebble [-c] dbdir\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("pebble: ")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
	}
	dbfile := flag.Arg(0)

	if !*createDB {
		_, err := os.Stat(dbfile)
		if err != nil {
			log.Fatal(err)
		}
	}
	db, err := pebble.Open(dbfile, nil)
	if err != nil {
		log.Fatal(err)
	}

	s := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "> ")
		if !s.Scan() {
			break
		}
		line := s.Text()
		do(db, line)
	}
}

var (
	sync   = &pebble.WriteOptions{Sync: true}
	noSync = &pebble.WriteOptions{Sync: false}
)

func do(db *pebble.DB, line string) {
	x, err := parser.ParseExpr(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return
	}

	call, ok := x.(*ast.CallExpr)
	if !ok {
		fmt.Fprintf(os.Stderr, "not a call expression\n")
		return
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		fmt.Fprintf(os.Stderr, "call of non-identifier\n")
		return
	}
	switch id.Name {
	default:
		fmt.Fprintf(os.Stderr, "unknown operation %s\n", id.Name)

	case "get", "hex", "list":
		key, end, ok := getRange(id.Name, call.Args, id.Name == "list")
		if !ok {
			return
		}
		if end == nil {
			val, closer, err := db.Get(key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				return
			}
			defer closer.Close()
			if id.Name == "hex" {
				fmt.Printf("%s\n", hex.Dump(val))
				return
			}
			fmt.Printf("%s\n", decode(val))
			return
		}

		iter, err := db.NewIter(&pebble.IterOptions{LowerBound: key, UpperBound: end})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
		defer iter.Close()
		for iter.First(); iter.Valid(); iter.Next() {
			switch id.Name {
			case "get":
				fmt.Printf("%s: %s\n", decode(iter.Key()), decode(iter.Value()))
			case "hex":
				fmt.Printf("%s:\n%s\n", decode(iter.Key()), hex.Dump(iter.Value()))
			case "list":
				fmt.Printf("%s\n", decode(iter.Key()))
			}
		}

	case "mvprefix":
		if len(call.Args) != 2 {
			fmt.Fprintf(os.Stderr, "usage: mvprefix(old, new)\n")
			return
		}
		old, ok := getEnc(call.Args[0])
		if !ok {
			return
		}
		new, ok := getEnc(call.Args[1])
		if !ok {
			return
		}
		iter, err := db.NewIter(&pebble.IterOptions{LowerBound: old})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
		defer iter.Close()
		var last []byte
		for iter.First(); iter.Valid(); iter.Next() {
			if !bytes.HasPrefix(iter.Key(), old) {
				break
			}
			if err := db.Set(append(new, iter.Key()[len(old):]...), iter.Value(), noSync); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				return
			}
			last = bytes.Clone(iter.Key())
		}
		if last != nil {
			if err := db.DeleteRange(old, iter.Key(), sync); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			}
		}

	case "set":
		if len(call.Args) != 2 {
			fmt.Fprintf(os.Stderr, "usage: set(key, value)\n")
			return
		}
		key, ok := getEnc(call.Args[0])
		if !ok {
			return
		}
		val, ok := getEnc(call.Args[1])
		if !ok {
			return
		}
		if err := db.Set(key, val, sync); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}

	case "delete":
		key, end, ok := getRange(id.Name, call.Args, false)
		if !ok {
			return
		}
		if end == nil {
			if err := db.Delete(key, sync); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
			}
			return
		}
		if err := db.DeleteRange(key, end, sync); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}

	case "compact":
		if len(call.Args) != 0 {
			fmt.Fprintf(os.Stderr, "compact takes no arguments\n")
			return
		}
		if err := db.Compact(nil, ordered.Encode(ordered.Inf), false); err != nil {
			fmt.Fprintf(os.Stderr, "compact: %v\n", err)
			return
		}
		if err := db.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "compact: %v\n", err)
			return
		}
		if err := db.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "compact: %v\n", err)
			return
		}
		var err error
		db, err = pebble.Open("pebble.db", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "compact: %v\n", err)
			log.Fatal("cannot reopen database")
		}
	}
}

func getRange(name string, args []ast.Expr, forceRange bool) (lo, hi []byte, ok bool) {
	if forceRange && len(args) < 2 {
		fmt.Fprintf(os.Stderr, "need two arguments for key range in call to %s\n", name)
		return nil, nil, false
	}
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "too many arguments in call to %s", name)
		return nil, nil, false
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "no arguments in call to %s", name)
		return nil, nil, false
	}
	lo, ok = getEnc(args[0])
	if !ok {
		return nil, nil, false
	}
	if len(args) == 2 {
		hi, ok = getEnc(args[1])
		if !ok {
			return nil, nil, false
		}
	}
	return lo, hi, true
}

func getEnc(x ast.Expr) ([]byte, bool) {
	switch x := x.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			break
		}
		enc, err := strconv.Unquote(x.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid quoted string %s\n", x.Value)
			return nil, false
		}
		return []byte(enc), true

	case *ast.CallExpr:
		fn, ok := x.Fun.(*ast.Ident)
		if !ok || fn.Name != "o" {
			break
		}
		var list []any
		for _, arg := range x.Args {
			a, ok := getArg(arg, 0)
			if !ok {
				return nil, false
			}
			list = append(list, a)
		}
		return ordered.Encode(list...), true
	}

	fmt.Fprintf(os.Stderr, "argument %s must be quoted string or o(list)\n", gofmt(x))
	return nil, false
}

const (
	noRev = 1 << iota
	forceFloat64
)

func getArg(x ast.Expr, flags int) (any, bool) {
	switch x := x.(type) {
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING:
			v, err := strconv.Unquote(x.Value)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid quoted string %s\n", x.Value)
				return nil, false
			}
			return v, true
		case token.INT:
			if flags&forceFloat64 != 0 {
				f, err := strconv.ParseFloat(x.Value, 64)
				if err == nil {
					return f, true
				}
				break
			}
			i, err := strconv.ParseInt(x.Value, 0, 64)
			if err == nil {
				return i, true
			}
			u, err := strconv.ParseUint(x.Value, 0, 64)
			if err == nil {
				return u, true
			}

		case token.FLOAT:
			f, err := strconv.ParseFloat(x.Value, 64)
			if err == nil {
				return f, true
			}
		}

	case *ast.UnaryExpr:
		if x.Op == token.ADD || x.Op == token.SUB {
			sign := +1
			if x.Op == token.SUB {
				sign = -1
			}
			if id, ok := x.X.(*ast.Ident); ok && id.Name == "Inf" {
				if flags&forceFloat64 != 0 {
					return math.Inf(sign), true
				}
				fmt.Fprintf(os.Stderr, "must use float32(%s) or float64(%s)\n", gofmt(x), gofmt(x))
				return nil, false
			}
			if basic, ok := x.X.(*ast.BasicLit); ok {
				v, ok := getArg(basic, flags)
				if !ok {
					return nil, false
				}
				switch v := v.(type) {
				case int64:
					return v * int64(sign), true
				case uint64:
					if sign == -1 && v > 1<<63 {
						fmt.Fprintf(os.Stderr, "%s is out of range for int64", gofmt(x))
						return nil, false
					}
					return v * uint64(sign), true
				case float64:
					return v * float64(sign), true
				}
			}
		}

	case *ast.Ident:
		switch x.Name {
		case "Inf":
			if flags&forceFloat64 != 0 {
				return math.Inf(+1), true
			}
			return ordered.Inf, true
		case "NaN":
			if flags&forceFloat64 != 0 {
				return math.NaN(), true
			}
			fmt.Fprintf(os.Stderr, "must use float32(NaN) or float64(NaN)\n")
			return nil, false
		}

	case *ast.CallExpr:
		fn, ok := x.Fun.(*ast.Ident)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown call to %s\n", gofmt(x.Fun))
			return nil, false
		}
		if len(x.Args) != 1 {
			fmt.Fprintf(os.Stderr, "call to %s requires 1 argument\n", gofmt(x.Fun))
			return nil, false
		}
		switch fn.Name {
		default:
			fmt.Fprintf(os.Stderr, "unknown call to %s\n", fn.Name)
			return nil, false

		case "rev":
			if flags&noRev != 0 {
				fmt.Fprintf(os.Stderr, "invalid nested reverse\n")
				return nil, false
			}
			v, ok := getArg(x.Args[0], noRev)
			if !ok {
				return nil, false
			}
			return ordered.RevAny(v), true

		case "float32":
			v, ok := getArg(x.Args[0], noRev|forceFloat64)
			if !ok {
				return nil, false
			}
			return float32(v.(float64)), true

		case "float64":
			return getArg(x.Args[0], noRev|forceFloat64)
		}
	}
	fmt.Fprintf(os.Stderr, "invalid ordered value %s", gofmt(x))
	return nil, false
}

func decode(enc []byte) string {
	if s, err := ordered.DecodeFmt(enc); err == nil {
		return "o" + s
	}
	s := string(enc)
	if strconv.CanBackquote(s) {
		return "`" + s + "`"
	}
	return strconv.QuoteToGraphic(s)
}

var emptyFset = token.NewFileSet()

func gofmt(x ast.Expr) string {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, emptyFset, x)
	if err != nil {
		return fmt.Sprintf("?err: %s?", err)
	}
	return buf.String()
}
