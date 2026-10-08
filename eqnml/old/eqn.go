// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Eqnml converts an equation language similar to Unix's eqn(1)
// to [MathML Core] syntax that can be used in HTML pages.
//
// See the README at https://pkg.go.dev/rsc.io/eqnml/ for examples.
//
// [MathML Core]: https://www.w3.org/TR/mathml-core/
package eqnml

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CSS is the CSS style definitions needed for rendering the output of ToHTML.
var CSS string = css

var css = `/* eqnml css */
math.eqn mspace {width: 0.25em;}
math.eqn mspace.thin {width: 0.125em;}
math.eqn mtd.lcol { text-align: -webkit-left; text-align: -moz-left; }
math.eqn mtd.rcol { text-align: -webkit-right; text-align: -moz-right; }
/* end of eqnml css */
`

// ToHTML converts the equation language text to MathML Core
// markup or else reports parse errors.
//
// The result should be wrapped in <math></math> tags,
// possibly adding the display=block attribute to the opening tag.
// The page where the result is rendered should also include [CSS]
// in its style sheet.
//
// See https://pkg.go.dev/rsc.io/eqnml/ for a description of the language.
func ToHTML(text string) (string, error) {
	lx := &lexer{
		input:  text,
		file:   "",
		lineno: 1,
	}
	yyParse(lx)
	if lx.errbuf.Len() > 0 {
		return "", errors.New(lx.errbuf.String())
	}

	if lx.input != "" {
		return "", errors.New("did not consume entire equation")
	}
	return string(lx.math), nil
}

type line struct {
	file   string
	lineno int
}

func (l line) String() string {
	return fmt.Sprintf("%s:%d", l.file, l.lineno)
}

func (l line) Errorf(lx *lexer, format string, args ...interface{}) {
	fmt.Fprintf(&lx.errbuf, "%s: %s\n", l, fmt.Sprintf(format, args...))
}

type lexer struct {
	input  string
	file   string
	lineno int
	sym    string
	math   []byte
	errbuf bytes.Buffer
}

func re(s string) *regexp.Regexp {
	return regexp.MustCompile(`\A(?:` + s + `)`)
}

var tokens = []struct {
	re  *regexp.Regexp
	val int
	fn  func(*lexer, string, *yySymType)
}{
	{re(`[ \t\n]`), -1, nil},
	{re(`{`), '{', nil},
	{re(`}`), '}', nil},
	{re(`~`), '~', nil},
	{re(`\^`), '^', nil},
	{re(`&`), '&', nil},
	{re(`\\\\`), BACKSLASH2, nil},
	{re(`matrix`), MATRIX, nop},
	//{re(`vmatrix`), VMATRIX, nop},
	{re(`mrow`), MROW, nop},
	{re(`above`), ABOVE, nop},
	{re(`sub`), SUB, nop},
	{re(`sup`), SUP, nop},
	{re(`op[\[\]()|{}/]`), CONTIG, makeOp},
	{re(`"[^"]+"`), QTEXT, makeQtext},
	{re(`[^ \t\n{}"~^]+`), CONTIG, makeContig},
}

func makeQtext(lx *lexer, s string, yy *yySymType) {
	yy.s = s[1 : len(s)-1]
}

func makeOp(lx *lexer, s string, yy *yySymType) {
	yy.b = []byte("<mo>" + s[len("op"):] + "</mo>")
}

func makeContig(lx *lexer, s string, yy *yySymType) {
	var buf []byte
	kind := ""
	for _, r := range s {
		if r == '-' {
			r = '\u2212' // math minus sign
		}
		k := runeKind(r)
		if k != kind {
			if kind != "" {
				buf = append(buf, "</"...)
				buf = append(buf, kind...)
				buf = append(buf, '>')
			}
			kind = k
			buf = append(buf, "<"...)
			buf = append(buf, kind...)
			buf = append(buf, '>')
		}
		buf = utf8.AppendRune(buf, r)
	}
	if kind != "" {
		buf = append(buf, "</"...)
		buf = append(buf, kind...)
		buf = append(buf, '>')
	}
	yy.b = buf
}

func runeKind(r rune) string {
	if '0' <= r && r <= '9' || '₀' <= r && r <= '₉' || r == 'ₙ' || r == '⁰' || r == '¹' || r == '²' || r == '³' || '⁴' <= r && r <= '⁹' || r == '∞' {
		return "mn"
	}
	if unicode.IsLetter(r) || r == 'ⁿ' {
		return "mi"
	}
	if r == '+' || r == '\u2212' || r == '\u2299' || r == '=' {
		return "mo"
	}
	return "mtext"
}

func (lx *lexer) Lex(yy *yySymType) int {
	if len(lx.input) == 0 {
		return EOF
	}

	var (
		longest    string
		longestVal int
		longestFn  func(*lexer, string, *yySymType)
	)
	for _, tok := range tokens {
		s := tok.re.FindString(lx.input)
		if len(s) > len(longest) {
			longest = s
			longestVal = tok.val
			longestFn = tok.fn
		}
	}
	if longest == "" {
		lx.Error(fmt.Sprintf("lexer stuck at %.10q", lx.input))
		return -1
	}
	yy.line = lx.line()
	if longestFn != nil {
		lx.sym = longest
		longestFn(lx, longest, yy)
	}
	lx.input = lx.input[len(longest):]
	lx.lineno += strings.Count(longest, "\n")
	if longestVal < 0 {
		// skip
		return lx.Lex(yy)
	}
	return longestVal
}

func (lx *lexer) Error(s string) {
	lx.line().Errorf(lx, "%s near %s", s, lx.sym)
}

func (lx *lexer) line() line {
	return line{lx.file, lx.lineno}
}

func nop(*lexer, string, *yySymType) {
	// having a function in the table
	// will make the lexer save the string
	// for use in error messages.
	// nothing more to do.
}

type Column struct {
	Align string
	Pile  [][]byte
}

func matrix(cols []Column) []byte {
	if len(cols) == 0 {
		return []byte("<mtable></mtable>")
	}
	n := 0
	for _, c := range cols {
		n = max(n, len(c.Pile))
	}
	var buf bytes.Buffer
	buf.WriteString("<mtable>")
	for i := range n {
		buf.WriteString("<mtr>")
		for _, c := range cols {
			buf.WriteString("<mtd")
			if c.Align != "col" {
				buf.WriteString(" class='")
				buf.WriteString(c.Align)
				buf.WriteString("'")
			}
			buf.WriteString(">")
			if i < len(c.Pile) {
				buf.Write(c.Pile[i])
			}
			buf.WriteString("</mtd>")
		}
		buf.WriteString("</mtr>")
	}
	buf.WriteString("</mtable>")
	return buf.Bytes()
}

func vmatrix(align string, rows [][][]byte) []byte {
	if len(rows[len(rows)-1]) == 0 {
		rows = rows[:len(rows)-1]
	}
	var buf bytes.Buffer
	buf.WriteString("<mtable>")
	for _, row := range rows {
		buf.WriteString("<mtr>")
		for i, col := range row {
			buf.WriteString("<mtd")
			if i < len(align) && (align[i] == 'r' || align[i] == 'l' || align[i] == 'c') {
				buf.WriteString(" class='")
				buf.WriteByte(align[i])
				buf.WriteString("col'")
			}
			buf.WriteString(">")
			buf.Write(col)
			buf.WriteString("</mtd>")
		}
		buf.WriteString("</mtr>")
	}
	buf.WriteString("</mtable>")
	return buf.Bytes()
}
