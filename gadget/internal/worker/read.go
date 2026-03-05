// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type readFileArgs struct {
	File string `json:"file" jsonschema:"path to file to read"`
}

type readFileReply struct {
	Content string `json:"content" jsonschema:"file content"`
}

func registerReadFile(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_file",
		Description: "read file",
	}, readFile)
}

func readFile(ctx context.Context, req *mcp.CallToolRequest, args readFileArgs) (*mcp.CallToolResult, readFileReply, error) {
	fmt.Fprintf(os.Stderr, ">> read_file %s\n", args.File)
	data, err := os.ReadFile(args.File)
	if err != nil {
		return nil, readFileReply{}, err
	}
	return nil, readFileReply{Content: string(data)}, nil
}
