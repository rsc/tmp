// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate cp $GOROOT/lib/wasm/wasm_exec.js .
//go:generate env GOOS=js GOARCH=wasm go build -o main.wasm

package main

import "syscall/js"

func main() {
	println("Go starting")
	js.Global().Get("window").Set("gocallback", js.FuncOf(func(this js.Value, args []js.Value) any {
		println(args[0].String())
		return js.ValueOf(Rot13(args[0].String()))
	}))
	select {}
}

func Rot13(s string) string {
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
