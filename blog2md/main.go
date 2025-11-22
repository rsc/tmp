// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	for _, arg := range os.Args[1:] {
		filepath.Walk(arg, func(path string, info fs.FileInfo, err error) error {
			var out bytes.Buffer
			if !strings.HasSuffix(path, ".article") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				log.Fatal(err)
			}
			lines := strings.Split(string(data), "\n")
			if len(lines) < 10 || !strings.HasPrefix(lines[0], "# ") {
				log.Fatalf("%s: malformed article start", path)
			}
			fmt.Fprintf(&out, "---\ntitle: %s\n", yamlEscape(lines[0][2:]))
			date, ok := parseTime(lines[1])
			if !ok {
				log.Fatalf("%s: bad date: %v", path, lines[1])
			}
			if h, m, s := date.Clock(); h != 11 || m != 0 || s != 0 {
				fmt.Fprintf(&out, "date: %s\n", date.Format("2006-01-02T15:04:05Z"))
			} else {
				fmt.Fprintf(&out, "date: %s\n", date.Format("2006-01-02"))
			}
			var meta bytes.Buffer
			lines = lines[2:]
			for ; len(lines) > 0 && lines[0] != ""; lines = lines[1:] {
				line := lines[0]
				if strings.HasPrefix(line, "Tags:") {
					fmt.Fprintf(&meta, "tags:\n")
					for _, f := range strings.Fields(line)[1:] {
						fmt.Fprintf(&meta, "- %s\n", yamlEscape(strings.TrimSuffix(f, ",")))
					}
					continue
				}
				if strings.HasPrefix(line, "Summary:") {
					fmt.Fprintf(&meta, "summary: %s\n", yamlEscape(strings.TrimSpace(strings.TrimPrefix(line, "Summary:"))))
					continue
				}
				if strings.HasPrefix(line, "OldURL: /") {
					old := strings.TrimPrefix(line, "OldURL: /")
					redir := []byte(fmt.Sprintf("---\nredirect: /blog/%s\n---\n", strings.TrimSuffix(filepath.Base(path), ".article")))
					err := os.WriteFile(filepath.Dir(path)+"/"+old+".md", redir, 0666)
					if err != nil {
						log.Fatalf("%s: writing redirect: %v", path, err)
					}
					continue
				}
				log.Fatalf("%s: unexpected line: %s", path, line)
			}
			haveAuthors := false
			for len(lines) > 0 && lines[0] == "" {
				lines = lines[1:]
				if len(lines) == 0 {
					log.Fatalf("%s: missing author", path)
				}
				if strings.HasPrefix(lines[0], "##") {
					break
				}
				if !haveAuthors {
					haveAuthors = true
					fmt.Fprintf(&out, "by:\n")
				}
				fmt.Fprintf(&out, "- %s\n", lines[0])
				lines = lines[1:]
				for len(lines) > 0 && lines[0] != "" {
					lines = lines[1:]
				}
			}
			out.Write(meta.Bytes())
			fmt.Fprintf(&out, "---\n\n")
			if len(lines) == 0 {
				log.Fatalf("%s: unexpected EOF", path)
			}
			if lines[0] == "##" {
				lines = lines[1:]
			}

			for _, line := range lines {
				if !strings.HasPrefix(line, ".") {
					fmt.Fprintf(&out, "%s\n", line)
					continue
				}
				f := strings.Fields(line)
				verb, args := f[0], f[1:]
				switch verb {
				case ".image":
					if len(args) == 1 {
						fmt.Fprintf(&out, "{{image %q}}\n", args[0])
					} else if len(args) == 3 && args[1] == "_" {
						fmt.Fprintf(&out, "{{image %q %s}}\n", args[0], args[2])
					} else if len(args) == 3 {
						fmt.Fprintf(&out, "{{image %q %s %s}}\n", args[0], args[2], args[1]) // url h w -> url w h
					} else {
						log.Fatalf("%s: malformed: %s\n", path, line)
					}

				case ".code", ".play":
					verb := verb[1:]
					if len(args) >= 1 && args[0] == "-edit" {
						args = args[1:]
					}
					end := ""
					if len(args) >= 1 && args[0] == "-numbers" {
						end = " 0"
						args = args[1:]
					}
					if len(args) == 1 {
						fmt.Fprintf(&out, "{{%s %q%s}}\n", verb, args[0], end)
						break
					}
					if len(args) > 1 && strings.HasPrefix(args[1], "/") {
						addr := strings.Join(args[1:], " ")
						if strings.HasSuffix(addr, "/,") {
							fmt.Fprintf(&out, "{{%s %q %#q `$`%s}}\n", verb, args[0], addr[:len(addr)-1], end)
							break
						}
						if strings.HasSuffix(addr, "/,$") {
							fmt.Fprintf(&out, "{{%s %q %#q `$`%s}}\n", verb, args[0], addr[:len(addr)-2], end)
							break
						}
						if i := strings.Index(addr, "/,/"); i >= 0 {
							fmt.Fprintf(&out, "{{%s %q %#q %#q%s}}\n", verb, args[0],
								addr[:i+1], addr[i+2:], end)
							break
						}
						if strings.HasSuffix(addr, "/") {
							fmt.Fprintf(&out, "{{%s %q %#q%s}}\n", verb, args[0],
								addr, end)
							break
						}
					}
					log.Fatalf("%s: malformed: %s\n", path, line)

				case ".iframe":
					if len(args) != 3 {
						log.Fatalf("%s: malformed: %s\n", path, line)
					}
					if strings.HasPrefix(args[0], "//") {
						args[0] = "https:" + args[0]
					}
					if "520" <= args[2] && args[2] <= "560" {
						fmt.Fprintf(&out, "{{video %q}}\n", args[0])
					} else {
						fmt.Fprintf(&out, "{{video %q %s %s}}\n", args[0], args[2], args[1]) // url h w -> url w h
					}

				case ".html":
					if len(args) != 1 {
						log.Fatalf("%s: malformed: %s\n", path, line)
					}
					fmt.Fprintf(&out, "{{rawhtml (file %q)}}\n", args[0])

				default:
					log.Fatalf("%s: unknown verb %s\n", path, verb)
				}
				_ = args
			}

			err = os.WriteFile(strings.TrimSuffix(path, ".article")+".md", out.Bytes(), 0666)
			if err != nil {
				log.Fatalf("%s: %v", path, err)
			}
			println("did", path)
			return nil
		})
	}
}

func parseTime(text string) (t time.Time, ok bool) {
	t, err := time.Parse("15:04 2 Jan 2006", text)
	if err == nil {
		return t, true
	}
	t, err = time.Parse("2 Jan 2006", text)
	if err == nil {
		// at 11am UTC it is the same date everywhere
		t = t.Add(time.Hour * 11)
		return t, true
	}
	return time.Time{}, false
}

func yamlEscape(s string) string {
	if strings.ContainsAny(s, ":") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
