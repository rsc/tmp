// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) == 1 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		md, err := html2md("<stdin>", string(data))
		if err != nil {
			log.Fatalf("<stdin>: convert: %v", err)
		}
		md = strings.TrimRight(md, "\n") + "\n"
		os.Stdout.WriteString(md)
		return
	}

	for _, arg := range os.Args[1:] {
		filepath.Walk(arg, func(path string, info fs.FileInfo, err error) error {
			if !strings.HasSuffix(path, ".html") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				log.Fatal(err)
			}
			var buf bytes.Buffer
			if bytes.HasPrefix(data, []byte("<!--{")) {
				i := bytes.Index(data, []byte("}-->"))
				if i < 0 {
					log.Fatalf("%s: missing end of JSON", path)
				}
				var meta map[string]interface{}
				err := json.Unmarshal(data[4:i+1], &meta)
				if err != nil {
					log.Fatalf("%s: unmarshal JSON: %v", path, err)
				}

				delete(meta, "Template") // template always on for markdown
				for k, v := range meta {
					delete(meta, k)
					meta[strings.ToLower(k)] = v
				}
				out, err := yaml.Marshal(meta)
				if err != nil {
					log.Fatalf("%s: marshal YAML: %v", path, err)
				}
				buf.WriteString("---\n")
				buf.Write(out)
				buf.WriteString("---\n\n")
				data = data[i+4:]
			}

			md, err := html2md(path, string(data))
			if err != nil {
				log.Printf("%s: convert: %v", path, err)
				return nil
			}
			md = strings.TrimRight(md, "\n") + "\n"
			buf.WriteString(md)

			err = os.WriteFile(strings.TrimSuffix(path, ".html")+".md", buf.Bytes(), 0666)
			if err != nil {
				log.Fatalf("%s: %v", path, err)
			}
			println("did", path)
			return nil
		})
	}
}
