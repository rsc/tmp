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

type writeFileArgs struct {
	File    string `json:"file" jsonschema:"path to file to write"`
	Content string `json:"content" jsonschema:"file content"`
}

type writeFileReply struct {
	Size int64 `json:"size" jsonschema:"size of file written"`
}

func registerWriteFile(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "write_file",
		Description: "write file",
	}, writeFile)
}

func writeFile(ctx context.Context, req *mcp.CallToolRequest, args writeFileArgs) (*mcp.CallToolResult, writeFileReply, error) {
	fmt.Fprintf(os.Stderr, ">> write_file %s\n", args.File)
	err := os.WriteFile(args.File, []byte(args.Content), 0666)
	if err != nil {
		return nil, writeFileReply{}, err
	}
	return nil, writeFileReply{Size: int64(len(args.Content))}, nil
}
