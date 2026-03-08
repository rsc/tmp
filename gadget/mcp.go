// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"
)

// startWorker starts the worker MCP subprocess and returns
// the list of supplied tools as well as a shutdown function.
// The caller is responsible for calling shutdown when the
// tools are no longer needed.
func startWorker(ctx context.Context) (err error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "gadget",
		Version: "v1.0.0",
	}, nil)

	defer func() {
		if err != nil {
			err = fmt.Errorf("starting MCP worker: %v", err)
		}
	}()

	// MCP worker is gadget -runmcpworker.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("box", exe, "-runmcpworker")
	cmd.Stderr = os.Stderr
	transport := &mcp.CommandTransport{
		Command: cmd,
	}
	mcli, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	result, err := mcli.ListTools(ctx, nil)
	if err != nil {
		return err
	}

	for _, t := range result.Tools {
		tools = append(tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{{
				Name:                 t.Name,
				Description:          t.Description,
				ParametersJsonSchema: t.InputSchema,
				ResponseJsonSchema:   t.OutputSchema,
			}},
		})
		toolFuncs[t.Name] = func(ctx context.Context, args any) (any, error) {
			return mcpToolCall(ctx, mcli, t, args)
		}
	}
	return nil
}

// mcpToolCall invokes an MCP tool for the genai API.
func mcpToolCall(ctx context.Context, mcli *mcp.ClientSession, t *mcp.Tool, args any) (any, error) {
	result, err := mcli.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.Name,
		Arguments: args,
	})
	if err != nil {
		return nil, err
	}
	if result.IsError {
		// Extract error message from content
		for _, c := range result.Content {
			if text, ok := c.(*mcp.TextContent); ok {
				return nil, errors.New(text.Text)
			}
		}
		return nil, fmt.Errorf("tool returned unspecified error")
	}
	// The mcp.Content has a MarshalJSON method, so that's good enough for us.
	if len(result.Content) != 1 {
		return nil, fmt.Errorf("tool returned %d Content elements", len(result.Content))
	}
	return result.Content[0], nil
}
