// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eqnml

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

type pos struct {
	file string
	line int
}

type token struct {
	pos     pos
	s       string
	special bool
	builtin bool
}

func (t token) syntax(s string) bool {
	return t.special && t.s == s
}

func (t token) eof() bool {
	return t.s == ""
}

// isEscLetter reports whether c is a "letter" for purposes of appearing
// in an escape sequence. Like TeX, we only allow ASCII letters.
func isEscLetter(c rune) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

// isSpecial reports whether c is one of the special TeX characters (TeXbook p. 38).
func isSpecial(c rune) bool {
	switch c {
	case '\\', '{', '}', '$', '&', '#', '^', '_', '%', '~':
		return true
	}
	return false
}

// tokens breaks s into a sequence of TeX tokens.
// The start of s is recorded as having position pos.
func tokens(pos pos, s string) []token {
	var out []token
	for s != "" {
		c, n := utf8.DecodeRuneInString(s)
		if c == '%' { // comment to end of line
			// TODO: Does this count as a space? In math mode it probably doesn't matter.
			i := strings.Index(s, "\n")
			if i < 0 {
				s = ""
			} else {
				s = s[i+1:]
			}
			continue
		}
		if c == ' ' || c == '\t' {
			// NOTE: Perhaps TeX treats tabs differently, but for us, they're just spaces.
			s = s[1:]
			continue
		}
		if c == '\n' {
			pos.line++
			s = s[1:]
			continue
		}
		if c != '\\' {
			out = append(out, token{pos: pos, s: s[:n], special: isSpecial(c)})
			s = s[n:]
			continue
		}

		// \$ for some symbol $ or \word for some sequence of letters.
		c, n = utf8.DecodeRuneInString(s[1:])
		if c == '\n' {
			pos.line++
		}
		e := 1 + n
		if isEscLetter(c) {
			for {
				c, n := utf8.DecodeRuneInString(s[e:])
				if !isEscLetter(c) {
					break
				}
				e += n
			}
		}
		// Note: In the word case, we are supposed to consume
		// spaces that follow (but not include them in the token),
		// but since we're in math mode spaces will be ignored anyway.
		out = append(out, token{pos: pos, s: s[:e], special: true})
		s = s[e:]
	}
	return out
}

func newTexParser() *texParser {
	p := &texParser{
		defs: maps.Clone(defs),
	}
	p.push(tokens(pos{"preamble", 1}, preamble)...)
	t := p.next()
	if !t.eof() {
		fmt.Println("TOKEN", t)
		panic("preamble failed")
	}
	return p
}

type texParser struct {
	fwd   []token
	rev   []token
	text  string
	pos   pos
	defs  map[string]func(*texParser)
	undef bool
}

func (p *texParser) push(toks ...token) {
	for _, t := range slices.Backward(toks) {
		p.rev = append(p.rev, t)
	}
}

func (p *texParser) pop() token {
	if len(p.rev) > 0 {
		t := p.rev[len(p.rev)-1]
		p.rev = p.rev[:len(p.rev)-1]
		p.pos = t.pos
		return t
	}
	if len(p.fwd) > 0 {
		t := p.fwd[0]
		p.fwd = p.fwd[1:]
		p.pos = t.pos
		return t
	}
	return token{}
}

func (p *texParser) error(at pos, format string, args ...any) {
	panic(fmt.Sprintf("%s:%d: %s", at.file, at.line, fmt.Sprintf(format, args...)))
}

func (p *texParser) next() token {
	var start pos
	var def string
	const maxTry = 1000
	for try := 0; ; try++ {
		if try >= maxTry {
			p.error(start, "stuck in a loop at %s", def)
		}
		t := p.pop()
		if !t.special || t.s[0] != '\\' {
			return t
		}
		f, ok := p.defs[t.s]
		if !ok {
			if p.undef {
				return t
			}
			p.error(t.pos, "undefined: %s", t.s)
		}
		if try == 0 {
			start = t.pos
		}
		def = t.s
		f(p)
	}
}

func (p *texParser) arg() []token {
	t := p.pop()
	if !t.syntax("{") {
		return []token{t}
	}

	start := t.pos
	var out []token
	depth := 1
	for depth > 0 {
		t := p.pop()
		if t.eof() {
			p.error(start, "eof searching for closing brace")
		}
		if t.syntax("{") {
			depth++
		} else if t.syntax("}") {
			depth--
		}
		out = append(out, t)
	}
	return out[:len(out)-1]
}

var defs = map[string]func(*texParser){
	`\backslash`: defBackslash,
	`\{`:         defLbrace,
	`\}`:         defRbrace,
	`\%`:         defPercent,
	`\#`:         defSharp,
	`\_`:         defUnder,
	`\&`:         defAmp,
	`\def`:       defDef,
	`\builtin`:   defBuiltin, // eqnml extension
	`\$`:         defDollar,
	`\ `:         defSpace,
	`\circum`:    defCircum, // eqnml extension
}

func defBackslash(p *texParser) { p.push(token{pos: p.pos, s: `\`}) }
func defLbrace(p *texParser)    { p.push(token{pos: p.pos, s: `{`}) }
func defRbrace(p *texParser)    { p.push(token{pos: p.pos, s: `}`}) }
func defPercent(p *texParser)   { p.push(token{pos: p.pos, s: `%`}) }
func defSharp(p *texParser)     { p.push(token{pos: p.pos, s: `#`}) }
func defUnder(p *texParser)     { p.push(token{pos: p.pos, s: `_`}) }
func defAmp(p *texParser)       { p.push(token{pos: p.pos, s: `&`}) }
func defDollar(p *texParser)    { p.push(token{pos: p.pos, s: `$`}) }
func defSpace(p *texParser)     { p.push(token{pos: p.pos, s: ` `}) }
func defCircum(p *texParser)    { p.push(token{pos: p.pos, s: `^`}) }

func defDef(p *texParser) {
	name := p.pop()
	if !name.special || name.s[0] != '\\' {
		p.error(p.pos, `\def must be followed by \name`)
	}

	body := p.arg()
	narg := 0
	for len(body) == 1 && body[0].syntax("#") {
		narg++
		n := p.pop()
		if n.special || n.s != fmt.Sprint(narg) {
			p.error(p.pos, `bad \def arg, want #%d, not #%s`, narg, n.s)
		}
		body = p.arg()
	}
	p.defs[name.s] = func(p *texParser) {
		args := make([][]token, narg)
		for i := range narg {
			args[i] = p.arg()
		}
		for i, t := range slices.Backward(body) {
			if i > 0 && body[i-1].syntax("#") {
				if t.special || len(t.s) != 1 || t.s[0] < '1' || t.s[0] > '0'+byte(narg) {
					p.error(p.pos, "bad %s body: #%s (%d args)", name.s, t.s, narg)
				}
				for _, t := range slices.Backward(args[t.s[0]-'1']) {
					p.push(t)
				}
				continue
			}
			if t.syntax("#") {
				if i == len(body)-1 {
					p.error(p.pos, "bad %s body: trailing #", name.s)
				}
				continue
			}
			p.push(t)
		}
	}
}

func defBuiltin(p *texParser) {
	name := p.pop()
	if !name.special || name.s[0] != '\\' {
		p.error(p.pos, `\builtin must be followed by \name`)
	}
	name.special = false
	name.builtin = true
	p.push(name)
}
