package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var ranBenchmark bool

func BenchmarkRun(b *testing.B) {
	if ranBenchmark {
		b.Fatal("can only run benchmark once!")
	}
	ranBenchmark = true
	if b.N != 1 {
		b.Fatal("can only run one iteration!")
	}
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "../../../../../pkg/"+runtime.GOOS+"_"+runtime.GOARCH)
	if err := os.Chdir(wd + "/../internal/gc"); err != nil {
		b.Fatal(err)
	}
	os.Args = []string{"bootstrap6g", "-o", "_bench.o", "-I", root,
		"align.go",
		"builtin.go",
		"bv.go",
		"cgen.go",
		"closure.go",
		"const.go",
		"cplx.go",
		"dcl.go",
		"esc.go",
		"export.go",
		"fmt.go",
		"gen.go",
		"go.go",
		"gsubr.go",
		"init.go",
		"inl.go",
		"lex.go",
		"mparith2.go",
		"mparith3.go",
		"obj.go",
		"opnames.go",
		"order.go",
		"pgen.go",
		"plive.go",
		"popt.go",
		"racewalk.go",
		"range.go",
		"reflect.go",
		"reg.go",
		"select.go",
		"sinit.go",
		"subr.go",
		"swt.go",
		"syntax.go",
		"typecheck.go",
		"unsafe.go",
		"util.go",
		"walk.go",
		"y.go",
		"yymsg.go",
	}
	main()
	os.Remove("_bench.o")
}
