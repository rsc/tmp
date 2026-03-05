// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type runShellArgs struct {
	Command string `json:"command" jsonschema:"shell command text"`
}

type runShellReply struct {
	Error  string `json:"error,omitempty" jsonschema:"shell error status"`
	Output string `json:"output" jsonschema:"shell command output (combined standard error and standard output)"`
}

func registerShell(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_shell_command",
		Description: "run shell command",
	}, runShell)
}

func runShell(ctx context.Context, req *mcp.CallToolRequest, args runShellArgs) (*mcp.CallToolResult, runShellReply, error) {
	fmt.Fprintf(os.Stderr, ">> shell %s\n", args.Command)
	cmd := exec.Command("bash", "-c", args.Command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Start()
	if err != nil {
		return nil, runShellReply{}, err
	}
	err = cmd.Wait()
	reply := runShellReply{
		Output: out.String(),
	}
	if err != nil {
		reply.Error = err.Error()
	}
	return nil, reply, nil
}
