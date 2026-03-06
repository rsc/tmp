// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerEditFile(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "edit_file",
		Description: editDesc,
	}, editFileCall)
}

type editFileArgs struct {
	Files []editFile `json:"files" jsonschema:"files to edit, along with edits to apply"`
}

type editFile struct {
	File  string          `json:"file" jsonschema:"path to file to edit"`
	Edits []editFileChunk `json:"edits" jsonschema:"line-based edits to apply"`
}

type editedFile struct {
	File   string `json:"file" jsonschema:"path to file"`
	Status string `json:"status" jsonschema:"outcome of edit ('edited successfully' or error text)"`
}

type editFileReply struct {
	Files []editedFile `json:"files" jsonschema:"results of editing files"`
}

type editFileChunk struct {
	Old string `json:"old" jsonschema:"lines to search for"`
	New string `json:"new" jsonschema:"lines that replace old lines"`
}

var editDesc = `edit_file edits one or more files.

For each file, it is given the name of the file and one or more line-based
changes to apply. Each change consists of old lines and new lines.
The old lines must appear exactly once in the file.
They are located and replaced with the new lines.
The old and new texts are zero or more entire lines from the file,
not just fragments of lines.

Include enough context in old to uniquely identify a single run
of lines in the file.
`

type editReplace struct {
	start int
	end   int
	text  string
}

func editFileCall(ctx context.Context, req *mcp.CallToolRequest, args editFileArgs) (*mcp.CallToolResult, editFileReply, error) {
	var reply editFileReply
	const editOK = "edited successfully"
	for _, file := range args.Files {
		err := editOneFile(file)
		status := editOK
		if err != nil {
			status = "editing error (file not modified): " + err.Error()
		}
		reply.Files = append(reply.Files, editedFile{file.File, status})
	}
	// In common case of editing a single file, be clearer that an error occurred.
	// TODO: Enable if necessary.
	if false && len(reply.Files) == 1 && reply.Files[0].Status != editOK {
		return nil, editFileReply{}, fmt.Errorf("editing %s: %v", reply.Files[0].File, reply.Files[0].Status)
	}
	return nil, reply, nil
}

func editOneFile(file editFile) error {
	btext, err := os.ReadFile(file.File)
	if err != nil {
		return err
	}
	text := string(btext)
	var repls []editReplace
	for _, edit := range file.Edits {
		all := searchLines(text, edit.Old)
		if len(all) == 0 {
			return fmt.Errorf("edit file %s: no matches for old text:\n%s", edit.Old)
		}
		if len(all) > 1 {
			return fmt.Errorf("edit file %s: multiple matches for old text:\n%s", edit.Old)
		}
		if edit.New != "" && !strings.HasSuffix(edit.New, "\n") {
			edit.New += "\n"
		}
		all[0].text = edit.New
		repls = append(repls, all[0])
	}
	slices.SortFunc(repls, func(x, y editReplace) int {
		return x.start - y.start
	})
	var edited bytes.Buffer
	at := 0
	for _, r := range repls {
		if r.start < at {
			return fmt.Errorf("overlapping file edits")
		}
		edited.WriteString(text[at:r.start])
		edited.WriteString(r.text)
		at = r.end
	}
	edited.WriteString(text[at:])
	return os.WriteFile(file.File, edited.Bytes(), 0666)
}

func searchLines(text, find string) []editReplace {
	if find == "" {
		// Allow searching for empty text in empty file.
		if len(text) == 0 {
			return []editReplace{{start: 0, end: 0}}
		}
		// Otherwise fail.
		return nil
	}
	if !strings.HasSuffix(find, "\n") {
		find += "\n"
	}
	var all []editReplace
	if strings.HasPrefix(text, find) {
		all = append(all, editReplace{start: 0, end: 0 + len(find)})
	}
	find = "\n" + find
	start := 0
	for {
		i := strings.Index(text[start:], find)
		if i < 0 {
			break
		}
		all = append(all, editReplace{start: start + i + 1, end: start + i + len(find)})
		start += i + len(find) - 1
	}
	return all
}
