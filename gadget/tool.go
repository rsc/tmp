// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"reflect"

	"google.golang.org/genai"
	"rsc.io/tmp/gadget/internal/schema"
)

var tools []*genai.Tool

var toolFuncs = make(map[string]func(context.Context, any)(any, error))

type tool[Args, Reply any] func(context.Context, *Args) (*Reply, error)

func registerTool[Args, Reply any](name, desc string, fn tool[Args, Reply]) {
	tools = append(tools, &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name: name,
			Description: desc,
			Parameters: mustType[*Args](),
			Response: mustType[*Reply](),
		},
	}})
	toolFuncs[name] = func(ctx context.Context, jsargs any) (any, error) {
		var args Args
		if err := schema.Unmarshal(jsargs, &args, "args"); err != nil {
			return nil, err
		}
		reply, err := fn(ctx, &args)
		if err != nil {
			return nil, err
		}
		jsreply, err := schema.Marshal(reply, "reply")
		if err != nil {
			return nil, err
		}
		return jsreply, nil
	}
}

func mustType[T any]() *genai.Schema {
	t, err := schema.Type(reflect.TypeFor[T]())
	if err != nil {
		panic(err)
	}
	return t
}
