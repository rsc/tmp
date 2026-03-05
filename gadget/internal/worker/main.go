// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Worker is an MCP server for operating system usage: file editing and running commands.
package worker

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Main() {
	log.SetFlags(0)
	log.SetPrefix("worker: ")

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "gadget-worker",
		Version: "v1.0.0",
	}, nil)

	registerReadFile(server)
	registerWriteFile(server)
	registerShell(server)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
