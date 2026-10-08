package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	bdata, err := os.ReadFile("/users/rsc/pub/markview.nvim/lua/markview/symbols.lua")
	if err != nil {
		log.Fatal(err)
	}
	data := string(bdata)

	syms, err := os.ReadFile("sym.go")
	if err != nil {
		log.Fatal(err)
	}

	out := new(bytes.Buffer)
	for line := range strings.Lines(string(syms)) {
		line = strings.TrimSpace(line)
		find := `["` + line + `"] = "`
		if i := strings.Index(data, find); i >= 0 {
			want := data[i+len(find):]
			want, _, _ = strings.Cut(want, "\n")
			want, _, _ = strings.Cut(want, `",`)
			fmt.Fprintf(out, "%q: %q,\n", line, want)
			continue
		}
		fmt.Fprintf(out, "%s\n", line)
	}

	if err := os.WriteFile("sym2.go", out.Bytes(), 0666); err != nil {
		log.Fatal(err)
	}
}
