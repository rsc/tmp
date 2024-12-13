// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate cp $GOROOT/lib/wasm/wasm_exec.js .
//go:generate env GOOS=js GOARCH=wasm go build -o main.wasm

package main

import (
	"bytes"
	_ "log"
	"strings"
	"syscall/js"

	"robpike.io/ivy/config"
	"robpike.io/ivy/exec"
	"robpike.io/ivy/parse"
	"robpike.io/ivy/run"
	"robpike.io/ivy/scan"
)

func main() {
	println("Go starting")

	js.Global().Get("window").Set("gocallback", js.FuncOf(func(this js.Value, args []js.Value) any {
		println("Callback", args[0].String())
		var conf config.Config
		var out bytes.Buffer
		conf.SetFormat("")
		conf.SetMaxBits(1e6)
		conf.SetMaxDigits(1e4)
		conf.SetMaxStack(100000)
		conf.SetOrigin(1)
		conf.SetPrompt("")
		conf.SetOutput(&out)
		conf.SetErrOutput(&out)
		context := exec.NewContext(&conf)
		scanner := scan.New(context, "input", strings.NewReader(args[0].String()))
		parser := parse.NewParser("input", scanner, context)
		out.Reset()
		run.Run(parser, context, false)
		return js.ValueOf(out.String())
	}))
	select {}
}

func Rot13(s string) string {
	BigStack(100000)
	b := []byte(s)
	for i, x := range b {
		if 'A' <= x && x <= 'M' || 'a' <= x && x <= 'm' {
			b[i] = x + 13
		} else if 'N' <= x && x <= 'Z' || 'n' <= x && x <= 'z' {
			b[i] = x - 13
		}
	}
	return string(b)
}

func BigStack(n int) {
	if n > 0 {
		BigStack(n - 1)
	}
}
