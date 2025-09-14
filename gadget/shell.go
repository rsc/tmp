// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os/exec"
	"bytes"
	"os"
)

func registerShell() {
	registerTool("run_shell_command", "run shell command", runShell)
}

type runShellArgs struct {
	Command string `tool:"shell command text"`
}

type runShellReply struct {
	Error string `tool:"#optional shell error status" json:",omitempty"`
	Output string `tool:"shell command output (combined standard error and standard output)"`
}

func runShell(ctx context.Context, args *runShellArgs) (*runShellReply, error) {
	fmt.Fprintf(os.Stderr, ">> shell %s\n", args.Command)
	cmd := exec.Command("bash", "-c", args.Command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	err = cmd.Wait()
	reply := &runShellReply{
		Output: out.String(),
	}
	if err != nil {
		reply.Error = err.Error()
	}
	return reply, nil
}
