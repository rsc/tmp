// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Unsafeconv finds unsafe conversions that might be made better
// with the proposals for unsafe.Slice and (*[10]int)(x[:]).
// It also finds those that won't fit the mold.
//
// Usage:
//	unsafeconv pkgs...
//
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"os"

	"golang.org/x/tools/go/packages"
)

func main() {
	cfg := packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Fset: token.NewFileSet(),
	}

	pkgs, err := packages.Load(&cfg, os.Args[1:]...)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				/*
					if ptr, siz, ok := toUnsafeSlice(n, p); ok {
						fmt.Printf("%s:%d: unsafe.Slice(%s, %s)\n\t%s\n",
							file.Name, fset.Position(n.Pos()).Line,
							show(ptr),
							show(siz),
							show(n))
						return false // do not process conversion inside
					}

					if slice, ok := n.(*ast.SliceExpr); ok {
						if _, typ, ok := toUnsafeArray(slice.X, p); ok {
							fmt.Printf("%s:%d otherslice %s\n\t%s\n", file.Name, fset.Position(n.Pos()).Line, show(typ), show(n))
							return false // do not process conversion inside
						}
					}
				*/

				if checkUnsafeSlice(n, p) {
					return false
				}

				checkUnsafeArray(n, p)

				return true
			})
		}
	}
}

var gofmtBuf bytes.Buffer

func gofmt(p *packages.Package, n interface{}) string {
	gofmtBuf.Reset()
	err := printer.Fprint(&gofmtBuf, p.Fset, n)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return gofmtBuf.String()
}

func show(p *packages.Package, n ast.Node, format string, args ...interface{}) {
	pos := p.Fset.Position(n.Pos())
	fmt.Printf("%s:%d: %s\n\t%s\n", pos.Filename, pos.Line, fmt.Sprintf(format, args...), gofmt(p, n))
}

func checkUnsafeSlice(n ast.Node, p *packages.Package) bool {
	slice, ok := n.(*ast.SliceExpr)
	if !ok {
		return false
	}
	tv := p.TypesInfo.Types[slice]
	if tv.Type == nil || !tv.IsValue() {
		show(p, n, "mistyped")
		return false
	}
	tslice, ok := tv.Type.(*types.Slice)
	if !ok {
		return false
	}
	call, ok := slice.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	paren, ok := call.Fun.(*ast.ParenExpr)
	if !ok {
		return false
	}
	_, ok = paren.X.(*ast.StarExpr)
	if !ok {
		return false
	}
	tv = p.TypesInfo.Types[paren.X]
	if tv.Type == nil || !tv.IsType() {
		show(p, n, "mistyped")
		return false
	}
	tptr, ok := tv.Type.(*types.Pointer)
	if !ok {
		show(p, n, "mistyped")
		return false
	}
	_, ok = tptr.Elem().(*types.Array)
	if !ok {
		return false
	}

	// Found conversion to array pointer type.
	// Now print something about it no matter what.

	// Unwrap inner unsafe.Pointer conversion.
	arg := call.Args[0]
	if call, ok := arg.(*ast.CallExpr); ok && len(call.Args) == 1 {
		ptv := p.TypesInfo.Types[call.Fun]
		if ptv.Type != nil && ptv.IsType() && ptv.Type.String() == "unsafe.Pointer" {
			arg = call.Args[0]
		}
	}

	argtv := p.TypesInfo.Types[arg]
	if argtv.Type == nil || !argtv.IsValue() {
		show(p, n, "mistyped")
		return true
	}
	tptr, ok = argtv.Type.(*types.Pointer)
	if !ok {
		show(p, n, "non-pointer")
		return true
	}
	if tptr.Elem() != tslice.Elem() {
		show(p, n, "slice-convert %v to %v: slice-elem-mismatch", tptr, tslice)
		return true
	}

	show(p, n, "slice-convert %v to %v: valid", tptr, tslice)
	return true
}

func checkUnsafeArray(n ast.Node, p *packages.Package) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return
	}
	paren, ok := call.Fun.(*ast.ParenExpr)
	if !ok {
		return
	}
	star, ok := paren.X.(*ast.StarExpr)
	if !ok {
		return
	}
	tv := p.TypesInfo.Types[paren.X]
	if tv.Type == nil || !tv.IsType() {
		show(p, n, "missing or bad type: %v", gofmt(p, star))
		return
	}
	tptr, ok := tv.Type.(*types.Pointer)
	if !ok {
		println("MISSING OR BAD POINTER TYPE", gofmt(p, star))
		return
	}
	tarr, ok := tptr.Elem().(*types.Array)
	if !ok {
		return
	}

	// Found conversion to array pointer type.
	// Now print something about it no matter what.

	// Unwrap inner unsafe.Pointer conversion.
	arg := call.Args[0]
	if call, ok := arg.(*ast.CallExpr); ok && len(call.Args) == 1 {
		ptv := p.TypesInfo.Types[call.Fun]
		if ptv.Type != nil && ptv.IsType() && ptv.Type.String() == "unsafe.Pointer" {
			arg = call.Args[0]
		}
	}

	argtv := p.TypesInfo.Types[arg]
	if argtv.Type == nil || !argtv.IsValue() {
		show(p, n, "mistyped")
		return
	}
	argtyp := argtv.Type

	// Look for &x[i].
	addr, ok := arg.(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		show(p, n, "array-convert %v to %v: non-addr-of", argtyp, tptr)
		return
	}

	index, ok := addr.X.(*ast.IndexExpr)
	if !ok {
		show(p, n, "array-convert %v to %v: addr-of-non-index", argtyp, tptr)
		return
	}
	tv = p.TypesInfo.Types[index.X]
	if tv.Type == nil || !tv.IsValue() {
		show(p, n, "mistyped")
		return
	}
	tslice, ok := tv.Type.(*types.Slice)
	if !ok {
		show(p, n, "array-convert %v to %v: addr-of-index-of-non-slice", argtyp, tptr)
		return
	}
	if tslice.Elem() != tarr.Elem() {
		show(p, n, "array-convert %v to %v: array-elem-mismatch", argtyp, tptr)
		return
	}

	show(p, n, "array-convert %v to %v: valid", argtyp, tptr)
}

/*
func toUnsafeSlice(n ast.Node, p *packages.Package) (ptr, siz ast.Node, ok bool) {
	slice, ok := n.(*ast.SliceExpr)
	if !ok || slice.Low != nil {
		return nil, nil, false
	}
	arg, typ, ok := toUnsafeArray(slice.X, p)
	if !ok {
		return nil, nil, false
	}

	conv, ok := arg.(*ast.CallExpr)
	if !ok || string(show(conv.Fun)) != "unsafe.Pointer" || len(conv.Args) != 1 {
		return nil, nil, false
	}
	elem := typ.(*ast.StarExpr).X.(*ast.ArrayType).Elt
	argtype := info.Types[conv.Args[0]].Type
	if argtype == nil || string(show(argtype)) != string(
	addr, ok := conv.Args[0].(*ast.UnaryExpr)
	if !ok || addr.Op != token.AND {
		return nil, nil, false
	}
	siz = slice.High
	if siz == nil {
		siz = typ.(*ast.StarExpr).X.(*ast.ArrayType).Len
	}
	return addr, siz, true
}
*/
