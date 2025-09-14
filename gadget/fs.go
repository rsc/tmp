// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
)

func registerReadFile() {
	registerTool("read_file", "read file", readFile)
}

type readFileArgs struct {
	File string `tool:"path to file to read"`
}

type readFileReply struct {
	Content string `tool:"file content"`
}

func readFile(ctx context.Context, args *readFileArgs) (*readFileReply, error) {
	fmt.Fprintf(os.Stderr, ">> read_file %s\n", args.File)
	data, err := os.ReadFile(args.File)
	if err != nil {
		return nil, err
	}
	reply := &readFileReply{
		Content: string(data),
	}
	return reply, nil
}

func registerWriteFile() {
	registerTool("write_file", "write file", writeFile)
}

type writeFileArgs struct {
	File string `tool:"path to file to write"`
	Content string `tool:"file content"`
}

type writeFileReply struct {
	Size int64 `tool:"size of file written"`
}

func writeFile(ctx context.Context, args *writeFileArgs) (*writeFileReply, error) {
	fmt.Fprintf(os.Stderr, ">> write_file %s\n", args.File)
	err := os.WriteFile(args.File, []byte(args.Content), 0666)
	if err != nil {
		return nil, err
	}
	reply := &writeFileReply{
		Size: int64(len(args.Content)),
	}
	return reply, nil
}
