// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate cp $GOROOT/lib/wasm/wasm_exec.js .
//go:generate env GOOS=js GOARCH=wasm go build -o main.wasm

package main

import (
	"bytes"
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

	var conf config.Config
	var out, errOut bytes.Buffer
	conf.SetFormat("")
	conf.SetMaxBits(1e6)
	conf.SetMaxDigits(1e4)
	conf.SetMaxStack(100000)
	conf.SetOrigin(1)
	conf.SetPrompt("")
	conf.SetOutput(&out)
	conf.SetErrOutput(&errOut)

	context := exec.NewContext(&conf)

	js.Global().Set("run", js.FuncOf(func(this js.Value, args []js.Value) any {
		scanner := scan.New(context, "input", strings.NewReader(args[0].String()))
		parser := parse.NewParser("input", scanner, context)
		out.Reset()
		errOut.Reset()
		ok := run.Run(parser, context, false)
		return js.ValueOf([]any{ok, out.String(), errOut.String()})
	}))

	select {}
}
