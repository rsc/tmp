// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strings"
)

type block interface {
	printBlock(p *printer)
}

type blocks []block

func (x blocks) printBlock(p *printer) {
	for i, b := range x {
		if i > 0 {
			p.printNL(true)
		}
		b.printBlock(p)
	}
}

type heading struct {
	level int
	id    string
	inner inlines
}

func (x heading) printBlock(p *printer) {
	p.buf.WriteString(strings.Repeat("#", x.level))
	p.buf.WriteString(" ")
	x.inner.printInline(p)
	if x.id != "" {
		p.buf.WriteString(" {#")
		p.buf.WriteString(x.id)
		p.buf.WriteString("}")
	}
	p.buf.WriteString("\n")
}

type tagBlock string

func (x tagBlock) printBlock(p *printer) {
	for _, line := range strings.Split(string(x), "\n") {
		line = strings.TrimRight(line, " \t")
		if line != "" {
			p.buf.Write(p.prefix)
			p.buf.WriteString(line)
		}
		p.printNL(true)
	}
}

type pre string

func (x pre) printBlock(p *printer) {
	s := string(x)
	s = strings.Trim(s, "\n")
	if s == "" {
		return
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, " \t")
		if line != "" {
			p.buf.Write(p.prefix)
			p.buf.WriteString("\t")
			p.buf.WriteString(line)
		}
		p.printNL(true)
	}
}

type defns []defn

type defn struct {
	dt inlines
	dd blocks
}

func (x defns) printBlock(p *printer) {
	for i, d := range x {
		if i > 0 {
			p.printNL(true)
		}
		d.dt.printInline(p)
		p.printNL(false)
		p.buf.WriteString(":   ")
		old := len(p.prefix)
		p.prefix = append(p.prefix, "    "...)
		d.dd.printBlock(p)
		p.prefix = p.prefix[:old]
	}
}

type list struct {
	num   int
	loose bool
	items []blocks
}

func (x list) printBlock(p *printer) {
	old := len(p.prefix)
	p.prefix = append(p.prefix, "    "...)
	for _, item := range x.items {
		if len(item) == 1 {
			if _, ok := item[0].(para); ok {
				continue
			}
		}
		x.loose = true
	}
	for i, item := range x.items {
		if i > 0 && x.loose {
			p.printNL(true)
		}
		p.buf.Write(p.prefix[:old])
		if x.num == 0 {
			p.buf.WriteString("  - ")
		} else {
			p.buf.WriteString(fmt.Sprintf("%2d. ", x.num+i))
		}
		item.printBlock(p)
	}
	p.prefix = p.prefix[:old]
}

type quote blocks

func (x quote) printBlock(p *printer) {
	old := len(p.prefix)
	p.prefix = append(p.prefix, "  > "...)
	blocks(x).printBlock(p)
	p.prefix = p.prefix[:old]
}

type inline interface {
	printInline(*printer)
}

type inlines []inline

func (x inlines) printInline(p *printer) {
	for _, inl := range x {
		inl.printInline(p)
	}
}

type para inlines

func (x para) printBlock(p *printer) {
	inlines(x).printInline(p)
	p.printNL(false)
}

type hardBreak struct{}

func (x hardBreak) printInline(p *printer) {
	p.needSpace = true
	p.maybeSpace()
	p.buf.WriteString("\\")
	p.printNL(false)
}

type emph inlines

func (x emph) printInline(p *printer) {
	p.maybeSpace()
	p.buf.WriteString("_")
	inlines(x).printInline(p)
	p.buf.WriteString("_")
}

type strong inlines

func (x strong) printInline(p *printer) {
	p.maybeSpace()
	p.buf.WriteString("**")
	p.trimSpace = true
	inlines(x).printInline(p)
	p.buf.WriteString("**")
}

type link struct {
	url   string
	inner inlines
}

const invalidAutoLink = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f <>"

func (x link) printInline(p *printer) {
	p.maybeSpace()
	if len(x.inner) == 1 {
		if t, ok := x.inner[0].(text); ok && strings.Contains(x.url, ":") && !strings.ContainsAny(x.url, invalidAutoLink) && (string(t) == x.url || "mailto:"+string(t) == x.url) {
			p.buf.WriteString("<")
			p.buf.WriteString(string(t))
			p.buf.WriteString(">")
			return
		}
	}
	p.buf.WriteString("[")
	x.inner.printInline(p)
	p.maybeSpace()
	p.buf.WriteString("](")
	p.buf.WriteString(x.url)
	p.buf.WriteString(")")
}

type image link

func (x image) printInline(p *printer) {
	p.maybeSpace()
	p.buf.WriteString("[")
	x.inner.printInline(p)
	p.maybeSpace()
	p.buf.WriteString("](")
	p.buf.WriteString(x.url)
	p.buf.WriteString(")")
}

type code string

func (x code) printInline(p *printer) {
	s := string(x)
	q := "`"
	for strings.Contains(s, q) {
		q += "`"
	}
	sep := ""
	if strings.HasPrefix(s, "`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, "`") || strings.HasSuffix(s, " ") {
		sep = " "
	}
	p.maybeSpace()
	p.buf.WriteString(q + sep + s + sep + q)
}

type tag string

func (x tag) printInline(p *printer) {
	p.maybeSpace()
	p.buf.WriteString(string(x))
}

type text string

func (x text) printInline(p *printer) {
	s := string(x)
	if p.trimSpace {
		s = strings.TrimSpace(s)
		p.trimSpace = false
	}
	for s != "" {
		var line string
		var haveNL bool
		line, s, haveNL = strings.Cut(s, "\n")
		trim := strings.TrimSpace(line)
		if trim == "" {
			p.needSpace = true
		} else {
			p.needSpace = p.needSpace || !strings.HasPrefix(line, trim)
			p.maybeSpace()
			trim = strings.Join(strings.Fields(trim), " ")
			escaper.WriteString(&p.buf, trim)
			p.needSpace = !strings.HasSuffix(line, trim)
		}
		if haveNL {
			p.printNL(false)
		}
	}
}

var escaper = strings.NewReplacer(`*`, `\*`, `_`, `\_`)

type printer struct {
	buf       bytes.Buffer
	needSpace bool
	trimSpace bool
	prefix    []byte
}

func (p *printer) printNL(force bool) {
	buf := p.buf.Bytes()
	i := len(buf)
	if i == 0 || buf[i-1] == '\n' {
		if !force {
			return
		}
		p.buf.Write(p.prefix)
		buf = p.buf.Bytes()
		i = len(buf)
	}
	j := bytes.LastIndexByte(buf, '\n')
	if len(buf)-(j+1) == len(p.prefix) && !force {
		return
	}
	for i > 0 && (buf[i-1] == ' ' || buf[i-1] == '\t') {
		i--
	}
	p.buf.Truncate(i)
	p.buf.WriteByte('\n')
	p.needSpace = false
}

func (p *printer) maybeSpace() {
	buf := p.buf.Bytes()
	if len(buf) >= 1 && buf[len(buf)-1] == '_' && (len(buf) < 2 || buf[len(buf)-2] != '\\') {
		buf = buf[:len(buf)-1]
	}
	if len(buf) >= 2 && buf[len(buf)-1] == '*' && buf[len(buf)-2] == '*' && (len(buf) < 3 || buf[len(buf)-3] != '\\') {
		buf = buf[:len(buf)-2]
	}
	if len(buf) == 0 || buf[len(buf)-1] == '\n' {
		p.buf.Write(p.prefix)
		p.needSpace = false
		return
	}
	if p.needSpace && buf[len(buf)-1] != ' ' {
		p.buf.WriteByte(' ')
	}
	p.needSpace = false
}

func mdprint(x block) string {
	if x == nil {
		return ""
	}
	var p printer
	x.printBlock(&p)
	return p.buf.String()
}
