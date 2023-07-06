// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
WIP of //go:fix inliner as described in
https://research.swtch.com/vgo-import#automatic_api_updates

Write a file GOROOT/math/minus.go containing

	package math

	//goo:fix
	func Minus(x float64) float64 {
		return -x
	}

Then `go run . x.go` should print

	x.go:7:1: found a doc comment
	x.go:7:1: found a //goo:fix
	x.go:13:6: found call to fixed function
	x.go:18:2: found call to fixed function

All that remains is to actually inline the function.
That is somewhat tricky since you have to somehow serialize the body
in a form that can be reconstructed, and then you have to reconstruct
it correctly.
*/

package main

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
)

var Analyzer = &analysis.Analyzer{
	Name:      "fixer",
	Doc:       "doc",
	Run:       run,
	FactTypes: []analysis.Fact{new(fixFact)},
}

type fixFact struct {
}

func (*fixFact) AFact() {}

func run(pass *analysis.Pass) (interface{}, error) {
	// Find and export declarations marked with //go:fix.
	println("RUN", pass.Pkg.Path())
	for _, f := range pass.Files {
		isMinus := false
		if pass.Pkg.Path() == "math" {
			isMinus = strings.HasSuffix(pass.Fset.Position(f.Package).Filename, "minus.go")
		}
		for _, decl := range f.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if doc := decl.Doc; doc != nil {
					pass.Reportf(doc.Pos(), "found a doc comment")
					for _, com := range doc.List {
						if strings.HasPrefix(com.Text, "//goo:fix") {
							f := strings.Fields(com.Text)
							if f[0] == "//goo:fix" {
								if isMinus {
									println("FOUND!")
								}
								pass.Reportf(com.Pos(), "found a //goo:fix")
								if len(f) > 1 {
									pass.Reportf(com.Pos(), "found a //goo:fix with args")
								}
								obj := pass.TypesInfo.Defs[decl.Name]
								if obj == nil {
									if isMinus {
										println("LOST!")
									}
									pass.Reportf(decl.Pos(), "lost info")
									continue
								}
								if isMinus {
									println("EXPORT")
								}
								pass.ExportObjectFact(obj, new(fixFact))
							}
						}
					}
				}
			}
		}
	}

	// Find calls of functions marked with //go:fix.
	var fact fixFact
	for _, f := range pass.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var obj types.Object
			switch x := call.Fun.(type) {
			case *ast.Ident:
				obj = pass.TypesInfo.Uses[x]
			case *ast.SelectorExpr:
				obj = pass.TypesInfo.Uses[x.Sel]
			}
			if obj == nil {
				return true
			}
			if pass.ImportObjectFact(obj, &fact) {
				pass.Reportf(call.Pos(), "found call to fixed function")
			}
			return true
		})
	}
	return nil, nil
}

func main() {
	multichecker.Main(Analyzer)
}
