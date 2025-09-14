// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "context"

func registerRot13() {
	registerTool("rot13", "translate text to rot13", rot13)
}

type rot13Args struct {
	Text string `tool:"text to be translated"`
}

type rot13Reply struct {
	Grkg string `tool:"rot13 of input text"`
}

func rot13(ctx context.Context, in *rot13Args) (*rot13Reply, error) {
	out := []byte(in.Text)
	for i, b := range out {
		if 'A' <= b && b <= 'M' || 'a' <= b && b <= 'm' {
			out[i] = b + 13
		} else if 'N' <= b && b <= 'Z' || 'n' <= b && b <= 'z' {
			out[i] = b - 13
		}
	}
	return &rot13Reply{Grkg: string(out)}, nil
}
