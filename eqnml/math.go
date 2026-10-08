// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package eqnml

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var mathOps map[string]func(*Parser) mathNode

func init() {
	mathOps = map[string]func(*Parser) mathNode{
		"^":               opSup,
		"_":               opSub,
		"{":               opGroup,
		`\msqrt`:          opMsqrt,
		`\root`:           opRoot,
		`\of`:             opOf,
		`\atop`:           opAtop,
		`\atopwithdelims`: opAtopwithdelims,
		`\over`:           opOver,
		`\vphantom`:       opVphantom,
		`\hphantom`:       opHphantom,
		`\mphantom`:       opMphantom,
		`\mrow`:           opMrow,
		`\munder`:         opMunder,
		`\mover`:          opMover,
		`\mtable`:         opMtable,
		`\left`:           opLeft,
		`\right`:          opRight,
		`\mspace`:         opMspace,
		`\mo`:             opMo,
		`\mocompact`:      opFont("<MO lspace='0' rspace='0'>", nil),
		`\moprefix`:       opFont("<MO form='prefix'>", nil),
		`\mostretchy`:     opFont("<MO stretchy=true>", nil),
		`\bf`:             opBf,
		`\nbf`:            opNbf,
		`\rm`:             opFont("<mtext>", nil),
		`\it`:             opFont("<mtext><i>", nil),
		`\code`:           opFont("<mtext><code>", nil),

		`\moserifitalic`:     opFont("<mo>", serifItalic),
		`\moserifbold`:       opFont("<mo>", serifBold),
		`\moserifbolditalic`: opFont("<mo>", serifBoldItalic),
		`\mosans`:            opFont("<mo>", sansNormal),
		`\mosansbold`:        opFont("<mo>", sansBold),
		`\mosansitalic`:      opFont("<mo>", sansItalic),
		`\mosansbolditalic`:  opFont("<mo>", sansBoldItalic),
		`\moscript`:          opFont("<mo>", scriptNormal),
		`\moscriptbold`:      opFont("<mo>", scriptBold),
		`\mofraktur`:         opFont("<mo>", frakturNormal),
		`\mofrakturbold`:     opFont("<mo>", frakturBold),
		`\momono`:            opFont("<mo>", monoNormal),
		`\modouble`:          opFont("<mo>", doubleBold),

		`\miserifitalic`:     opFont("<MI>", serifItalic),
		`\miserifbold`:       opFont("<MI>", serifBold),
		`\miserifbolditalic`: opFont("<MI>", serifBoldItalic),
		`\misans`:            opFont("<MI>", sansNormal),
		`\misansbold`:        opFont("<MI>", sansBold),
		`\misansitalic`:      opFont("<MI>", sansItalic),
		`\misansbolditalic`:  opFont("<MI>", sansBoldItalic),
		`\miscript`:          opFont("<MI>", scriptNormal),
		`\miscriptbold`:      opFont("<MI>", scriptBold),
		`\mifraktur`:         opFont("<MI>", frakturNormal),
		`\mifrakturbold`:     opFont("<MI>", frakturBold),
		`\mimono`:            opFont("<MI>", monoNormal),
		`\midouble`:          opFont("<MI>", doubleBold),

		`\mtext`:                opFont("<mtext>", nil),
		`\mtextserifitalic`:     opFont("<mtext>", serifItalic),
		`\mtextserifbold`:       opFont("<mtext>", serifBold),
		`\mtextserifbolditalic`: opFont("<mtext>", serifBoldItalic),
		`\mtextsans`:            opFont("<mtext>", sansNormal),
		`\mtextsansbold`:        opFont("<mtext>", sansBold),
		`\mtextsansitalic`:      opFont("<mtext>", sansItalic),
		`\mtextsansbolditalic`:  opFont("<mtext>", sansBoldItalic),
		`\mtextscript`:          opFont("<mtext>", scriptNormal),
		`\mtextscriptbold`:      opFont("<mtext>", scriptBold),
		`\mtextfraktur`:         opFont("<mtext>", frakturNormal),
		`\mtextfrakturbold`:     opFont("<mtext>", frakturBold),
		`\mtextmono`:            opFont("<mtext>", monoNormal),
		`\mtextdouble`:          opFont("<mtext>", doubleBold),
	}
}

type Parser struct {
	tex   *texParser
	stack []mathNode
	fmt   format
	nl    string
}

type format struct {
	force string
	remap *strings.Replacer
}

func NewParser() *Parser {
	tp := newTexParser()
	tp.undef = true
	return &Parser{tex: tp}
}

type mathNode interface {
	mathml(p *Parser, out *bytes.Buffer)
}

type mathText struct {
	kind mathTextKind
	s    string
}

type mathTextKind struct {
	tag   string
	attrs []string
}

func (k mathTextKind) eq(k2 mathTextKind) bool {
	return k.tag == k2.tag && slices.Equal(k.attrs, k2.attrs)
}

func (k mathTextKind) open(out *bytes.Buffer) {
	out.WriteString(k.tag)
	return

	out.WriteString("<")
	out.WriteString(k.tag)
	for _, a := range k.attrs {
		out.WriteString(" ")
		out.WriteString(a)
	}
	out.WriteString(">")
}

func (k mathTextKind) close(out *bytes.Buffer) {
	tag := k.tag
	// close all tags in reverse order
	for i := len(tag) - 1; i >= 0; i-- {
		if tag[i] == '<' {
			j := i + 1
			for j < len(tag) && tag[j] != '>' && tag[j] != ' ' {
				j++
			}
			out.WriteString("</")
			out.WriteString(tag[i+1 : j])
			out.WriteString(">")
		}
	}
	return

	out.WriteString("</")
	out.WriteString(k.tag)
	out.WriteString(">")
}

func (m *mathText) mathml(p *Parser, out *bytes.Buffer) {
	m.kind.open(out)
	out.WriteString(m.s)
	m.kind.close(out)
	out.WriteString(p.nl)
}

type mathGroup []mathNode

func (m mathGroup) mathml(p *Parser, out *bytes.Buffer) {
	if len(m) == 1 {
		m[0].mathml(p, out)
		return
	}
	out.WriteString("<mrow>")
	for _, x := range m {
		x.mathml(p, out)
	}
	out.WriteString("</mrow>")
}

type mrow []mathNode

func (m mrow) mathml(p *Parser, out *bytes.Buffer) {
	if len(m) == 1 {
		m[0].mathml(p, out)
		return
	}
	out.WriteString("<mrow>")
	for _, x := range m {
		x.mathml(p, out)
	}
	out.WriteString("</mrow>")
}

type msqrt struct {
	x mathNode
}

func (m *msqrt) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<msqrt>")
	m.x.mathml(p, out)
	out.WriteString("</msqrt>")
	out.WriteString(p.nl)
}

type msubsup struct {
	x   mathNode
	sub mathNode
	sup mathNode
}

type msubsupSpace struct{}

func (m *msubsupSpace) mathml(p *Parser, out *bytes.Buffer) {
	// msubsupSpace should only appear as msubsup.base,
	// and it is handled specially in msubsup.mathml.
	panic("misuse of msubsupSpace")
}

type mprime struct {
	n int
}

func (m *mprime) mathml(p *Parser, out *bytes.Buffer) {
	// mprime should only appear as msubsup.sup, and it is handled specially in msubsup.mathml.
	panic("misuse of mprime")
}

func primeStr(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "′"
	case 2:
		return "″"
	case 3:
		return "‴"
	}
	return strings.Repeat("⁗", n/4) + primeStr(n%4)
}

func (m *msubsup) mathml(p *Parser, out *bytes.Buffer) {
	tag := ""
	primes := ""
	if m.sup != nil {
		// TeXbook exercise 16.5 asks “Why do you think TeX treats \prime
		// as a large symbol that appears only in superscripts, instead of making it
		// a smaller symbol that has already been shifted up into the superscript position?”
		// The Unicode and MathML designers did not do this exercise,
		// so we cannot make a prime a superscript, so in particular
		// y_1' and y'_2 do not work correctly.
		if p, ok := m.sup.(*mprime); ok {
			primes = primeStr(p.n)
			m.sup = nil
		}
	}
	if m.sup != nil && m.sub != nil {
		tag = "msubsup"
	} else if m.sub != nil {
		tag = "msub"
	} else if m.sup != nil {
		tag = "msup"
	}

	if tag != "" {
		out.WriteString("<")
		out.WriteString(tag)
		out.WriteString(">")
	}
	if primes != "" {
		out.WriteString("<mrow>")
	}
	if _, ok := m.x.(*msubsupSpace); ok {
		// Top of stack is unstretched text or missing (empty stack).
		// Apply subscript/superscript to a fixed-size space instead.
		// Otherwise x₁ and y₁ or n² and N² have the subscripts/superscripts
		// at different depths/heights, which is incredibly annoying.
		// Use a short height for subscript-only, to avoid making boxes
		// taller than they need to be.
		height := "0em"
		if m.sup != nil {
			height = "0.66em"
		}
		out.WriteString("<mspace height='")
		out.WriteString(height)
		out.WriteString("' />")
	} else {
		m.x.mathml(p, out)
	}
	if primes != "" {
		out.WriteString("<mn>")
		out.WriteString(primes)
		out.WriteString("</mn>\n</mrow>")
	}

	if m.sub != nil {
		m.sub.mathml(p, out)
	}
	if m.sup != nil {
		m.sup.mathml(p, out)
	}
	if tag != "" {
		out.WriteString("</")
		out.WriteString(tag)
		out.WriteString(">")
		out.WriteString(p.nl)
	}
}

func (p *Parser) push(m mathNode) {
	p.stack = append(p.stack, m)
}

func (p *Parser) pop() mathNode {
	if len(p.stack) == 0 {
		return mathGroup{}
	}
	m := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	return m
}

func (p *Parser) maths() mathNode {
	old := p.stack
	p.stack = nil
	defer func() {
		p.stack = old
	}()

	for {
		t := p.tex.next()
		if t.eof() || t.syntax("}") {
			p.tex.push(t)
			break
		}
		p.tex.push(t)
		m := p.math()
		p.push(m)
	}
	return implicitGroup(p.stack)
}

func (p *Parser) take() mathNode {
	m := implicitGroup(p.stack)
	p.stack = nil
	return m
}

func implicitGroup(stack []mathNode) mathNode {
	stack = compact(stack)
	if len(stack) == 1 {
		return stack[0]
	}
	return mathGroup(stack)
}

func compact(stack []mathNode) []mathNode {
	w := 0
	for _, m := range stack {
		if t, ok := m.(*mathText); ok && w > 0 {
			if s, ok := stack[w-1].(*mathText); ok && s.kind.eq(t.kind) && !strings.HasPrefix(s.kind.tag, "<mi") && !strings.HasPrefix(s.kind.tag, "<mo") {
				s.s += t.s
				continue
			}
		}
		stack[w] = m
		w++
	}
	return stack[:w]
}

func (p *Parser) math() mathNode {
	t := p.tex.next()
	if p.fmt.force == "" {
		if t.s == "'" {
			return opPrime(p)
		}
		if t.s == "-" {
			t.s = "−"
		}
		if t.s == "*" {
			t.s = "∗"
		}
	}
	if unisub(t.s) != "" {
		return opUnisub(p, t)
	}
	if unisup(t.s) != "" {
		return opUnisup(p, t)
	}
	if !t.special && !t.builtin {
		return p.textNode(t.s)
	}
	f, ok := mathOps[t.s]
	if !ok {
		p.tex.error(p.tex.pos, "unknown math op %s", t.s)
	}
	return f(p)
}

func (p *Parser) popSubsup() *msubsup {
	if len(p.stack) > 0 {
		x := p.stack[len(p.stack)-1]
		switch x := x.(type) {
		case *msubsup:
			p.stack = p.stack[:len(p.stack)-1]
			return x
		case *mathText:
			if x.kind.tag != "mo" || len(x.kind.attrs) > 0 {
				break
			}
			p.stack = p.stack[:len(p.stack)-1]
			return &msubsup{x: x}
		default:
			p.stack = p.stack[:len(p.stack)-1]
			return &msubsup{x: x}
		}
	}

	// Top of stack is unstretched text or missing (empty stack).
	// Apply subscript/superscript to a fixed-size space instead.
	// Otherwise x₁ and y₁ or n² and N² have the subscripts/superscripts
	// at different depths/heights, which is incredibly annoying.
	return &msubsup{x: &msubsupSpace{}}
}

func opSup(p *Parser) mathNode {
	m := p.popSubsup()
	y := p.math()
	if m.sup != nil {
		p.tex.error(p.tex.pos, "double superscript")
	}
	m.sup = y
	return m
}

func opSub(p *Parser) mathNode {
	m := p.popSubsup()
	y := p.math()
	if m.sub != nil {
		p.tex.error(p.tex.pos, "double subscript")
	}
	m.sub = y
	return m
}

func opPrime(p *Parser) mathNode {
	n := 1
	for ; ; n++ {
		t := p.tex.next()
		if t.special || t.s != "'" {
			p.tex.push(t)
			break
		}
	}

	m := p.popSubsup()
	if m.sup != nil {
		p.tex.error(p.tex.pos, "double superscript")
	}
	m.sup = &mprime{n}
	return m
}

func unisup(s string) string {
	switch s {
	case "⁰":
		return "0"
	case "¹":
		return "1"
	case "²":
		return "2"
	case "³":
		return "3"
	case "⁴":
		return "4"
	case "⁵":
		return "5"
	case "⁶":
		return "6"
	case "⁷":
		return "7"
	case "⁸":
		return "8"
	case "⁹":
		return "9"
	}
	return ""
}

func unisub(s string) string {
	switch s {
	case "₀":
		return "0"
	case "₁":
		return "1"
	case "₂":
		return "2"
	case "₃":
		return "3"
	case "₄":
		return "4"
	case "₅":
		return "5"
	case "₆":
		return "6"
	case "₇":
		return "7"
	case "₈":
		return "8"
	case "₉":
		return "9"
	}
	return ""
}

func opUnisub(p *Parser, t token) mathNode {
	n := unisub(t.s)
	for {
		t := p.tex.next()
		c := unisub(t.s)
		if c == "" {
			p.tex.push(t)
			break
		}
		n += c
	}

	m := p.popSubsup()
	if m.sub != nil {
		p.tex.error(p.tex.pos, "double subscript")
	}
	m.sub = p.textNode(n)
	return m
}

func opUnisup(p *Parser, t token) mathNode {
	n := unisup(t.s)
	for {
		t := p.tex.next()
		c := unisup(t.s)
		if c == "" {
			p.tex.push(t)
			break
		}
		n += c
	}

	m := p.popSubsup()
	if m.sup != nil {
		p.tex.error(p.tex.pos, "double superscript")
	}
	m.sup = p.textNode(n)
	return m
}

func opFont(force string, remap *strings.Replacer) func(*Parser) mathNode {
	return func(p *Parser) mathNode {
		p.fmt.force = force
		p.fmt.remap = remap
		return p.math()
	}
}

func opCode(p *Parser) mathNode {
	p.fmt.force = "<mtext><code>"
	return p.math()
}

func opBf(p *Parser) mathNode {
	p.fmt.force = "<mtext><b>"
	return p.math()
}

func opNbf(p *Parser) mathNode {
	p.fmt.force = "<mn class='nbf'>"
	return p.math()
}

func opMo(p *Parser) mathNode {
	p.fmt.force = "<MO>"
	return p.math()
}

func opMoprefix(p *Parser) mathNode {
	p.fmt.force = "<MO form='prefix'>"
	return p.math()
}

func opGroup(p *Parser) mathNode {
	start := p.tex.pos
	format := p.fmt
	m := p.maths()
	t := p.tex.next()
	if t.eof() {
		p.tex.error(start, "eof in group")
	}
	if !t.syntax("}") { // should be impossible
		p.tex.error(t.pos, "unexpected token looking for } in group")
	}
	p.fmt = format
	return m
}

func opMsqrt(p *Parser) mathNode {
	return &msqrt{p.math()}
}

func (p *Parser) textNode(s string) *mathText {
	kind := p.textKind(s)
	if p.fmt.remap != nil {
		s = p.fmt.remap.Replace(s)
	}
	return &mathText{kind, s}
}

func (p *Parser) textKind(s string) mathTextKind {
	if p.fmt.force != "" {
		return mathTextKind{tag: p.fmt.force}
	}
	r, _ := utf8.DecodeRuneInString(s)
	switch {
	case unicode.IsLetter(r) || r == '∞':
		return mathTextKind{tag: "<mi>"}
	case unicode.IsDigit(r) || r == '.' || r == '¼' || r == '½' || r == '¾' || '⅐' <= r && r <= '⅟' || r == '↉': // TODO handle dot better
		return mathTextKind{tag: "<mn>"}
	}
	if s == "/" || s == "|" {
		return mathTextKind{tag: "<mn>"}
	}
	if s == " " {
		return mathTextKind{tag: "<mtext>"}
	}
	if strings.ContainsAny(s, "()⌊⌋⌈⌉[]{}") {
		return mathTextKind{tag: "<mo stretchy=false>", attrs: []string{"stretchy=false"}}
	}
	return mathTextKind{tag: "<mo>"}
}

type mfrac struct {
	top  mathNode
	bot  mathNode
	line string
}

func (m *mfrac) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mfrac")
	if m.line != "" {
		out.WriteString(" linethickness='")
		out.WriteString(m.line)
		out.WriteString("'")
	}
	out.WriteString(">")
	m.top.mathml(p, out)
	m.bot.mathml(p, out)
	out.WriteString("</mfrac>")
	out.WriteString(p.nl)
}

func opAtop(p *Parser) mathNode {
	top := p.take()
	bot := p.math()
	if _, ok := bot.(*mfrac); ok {
		p.tex.error(p.tex.pos, `double \atop or \over`)
	}
	return &mfrac{top: top, bot: bot, line: "0"}
}

func opAtopwithdelims(p *Parser) mathNode {
	top := p.take()
	left := p.math()
	right := p.math()
	bot := p.math()
	if _, ok := bot.(*mfrac); ok {
		p.tex.error(p.tex.pos, `double \atop or \over`)
	}
	return mrow{moStretchy(left), &mfrac{top: top, bot: bot, line: "0"}, moStretchy(right)}
}

func opOver(p *Parser) mathNode {
	top := p.take()
	bot := p.math()
	if _, ok := bot.(*mfrac); ok {
		p.tex.error(p.tex.pos, `double \atop or \over`)
	}
	return &mfrac{top: top, bot: bot}
}

type mroot struct {
	r mathNode
	x mathNode
}

func (m *mroot) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mroot>")
	m.x.mathml(p, out)
	m.r.mathml(p, out)
	out.WriteString("</mroot>")
	out.WriteString(p.nl)
}

func opRoot(p *Parser) mathNode {
	old := p.stack
	defer func() {
		p.stack = old
	}()
	p.stack = nil

	start := p.tex.pos
	for {
		t := p.tex.next()
		if t.eof() || t.syntax("}") {
			p.tex.error(start, `\root without \of`)
		}
		if t.syntax(`\of`) {
			break
		}
		p.tex.push(t)
		p.push(p.math())
	}
	r := p.take()
	x := p.maths()
	m := &mroot{
		r: r,
		x: x,
	}
	return m
}

func opOf(p *Parser) mathNode {
	p.tex.error(p.tex.pos, `\of without \root`)
	return nil // unreachable
}

func moStretchy(x mathNode) mathNode {
	t, ok := x.(*mathText)
	if !ok {
		return x
	}
	t.kind.tag = "<mo>"
	t.kind.attrs = nil
	return t
}

func opLeft(p *Parser) mathNode {
	old := p.stack
	defer func() {
		p.stack = old
	}()
	p.stack = nil

	x := p.math()
	t, ok := x.(*mathText)
	if !ok {
		p.tex.error(p.tex.pos, `invalid \left`)
	}
	if t.s != "." {
		t.kind.tag = "<mo>"
		t.kind.attrs = nil
		p.push(t)
	}

	start := p.tex.pos
	for {
		t := p.tex.next()
		if t.eof() || t.syntax("}") {
			p.tex.error(start, `\left without \right`)
		}
		if t.syntax(`\right`) {
			break
		}
		p.tex.push(t)
		p.push(p.math())
	}

	x = p.math()
	t, ok = x.(*mathText)
	if !ok {
		p.tex.error(p.tex.pos, `invalid \right`)
	}
	if t.s != "." {
		t.kind.tag = "<mo>"
		t.kind.attrs = nil
		p.push(t)
	}

	// By default, MathML grows the left and right delimiters to be the
	// height of the surrounding row, but we want the height of
	// the enclosed expression. Putting the operators inside an
	// explicit mrow limits the height to what is needed for the
	// enclosed expression.
	return mrow(p.stack)
}

func opRight(p *Parser) mathNode {
	p.tex.error(p.tex.pos, `mismatched \right`)
	return nil // not reached
}

type mphantom struct {
	x     mathNode
	class string
}

func opMphantom(p *Parser) mathNode {
	return &mphantom{x: p.math()}
}

func opVphantom(p *Parser) mathNode {
	return &mphantom{class: "vphantom", x: p.math()}
}

func opHphantom(p *Parser) mathNode {
	return &mphantom{class: "hphantom", x: p.math()}
}

func (m *mphantom) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mphantom")
	if m.class != "" {
		out.WriteString(" class='" + m.class + "'")
	}
	out.WriteString(">")
	m.x.mathml(p, out)
	out.WriteString("</mphantom>")
	out.WriteString(p.nl)
}

func opMrow(p *Parser) mathNode {
	m := p.math()
	if _, ok := m.(mrow); ok {
		return m
	}
	if g, ok := m.(mathGroup); ok {
		return mrow(g)
	}
	return mrow{m}
}

type mover struct {
	x mathNode
	a mathNode
}

func opMover(p *Parser) mathNode {
	return &mover{p.math(), p.math()}
}

func (m *mover) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mover>")
	m.x.mathml(p, out)
	m.a.mathml(p, out)
	out.WriteString("</mover>")
	out.WriteString(p.nl)
}

type munder struct {
	x mathNode
	a mathNode
}

func opMunder(p *Parser) mathNode {
	return &munder{p.math(), p.math()}
}

func (m *munder) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<munder>")
	m.x.mathml(p, out)
	m.a.mathml(p, out)
	out.WriteString("</munder>")
	out.WriteString(p.nl)
}

type mtable struct {
	rows [][]*mtd
}

func (m *mtable) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mtable>")
	out.WriteString(p.nl)
	for _, row := range m.rows {
		out.WriteString("<mtr>")
		for _, x := range row {
			x.mathml(p, out)
			out.WriteString(p.nl)
		}
		out.WriteString("</mtr>")
		out.WriteString(p.nl)
	}
	out.WriteString("</mtable>")
	out.WriteString(p.nl)
}

type mtd struct {
	align string
	x     mathNode
}

func (m *mtd) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mtd")
	if m.align != "" {
		fmt.Fprintf(out, " style='text-align: %s; text-align: -webkit-%s; text-align: -moz-%s;'", m.align, m.align, m.align)
	}
	out.WriteString(">")
	m.x.mathml(p, out)
	out.WriteString("</mtd>")
}

func opMtable(p *Parser) mathNode {
	before := p.stack
	format := p.fmt
	defer func() {
		p.stack = before
		p.fmt = format
	}()

	start := p.tex.pos
	t := p.tex.next()
	if !t.syntax("{") {
		p.tex.error(start, `\mtable without {`)
	}

	p.stack = nil
	var tab [][]*mtd
	var row []*mtd
	var fillbefore, fillafter bool
	cell := func() *mtd {
		td := &mtd{x: p.take()}
		switch {
		case fillbefore && fillafter:
			// already centered by default
		case fillbefore:
			td.align = "right"
		case fillafter:
			td.align = "left"
		}
		fillbefore = false
		fillafter = false
		p.fmt = format
		return td
	}
Loop:
	for {
		t := p.tex.next()
		if t.eof() {
			p.tex.error(start, `\mtable missing closing }`)
		}
		switch {
		case t.syntax(`\hfill`):
			if len(p.stack) == 0 {
				fillbefore = true
			} else {
				fillafter = true
			}
		case t.syntax("&"):
			row = append(row, cell())
		case t.syntax(`\cr`):
			row = append(row, cell())
			tab = append(tab, row)
			row = nil
		case t.syntax("}"):
			row = append(row, cell())
			tab = append(tab, row)
			break Loop
		default:
			p.tex.push(t)
			p.push(p.math())
		}
	}

	return &mtable{rows: tab}
}

type mspace struct {
	height string
	width  string
}

func (m *mspace) mathml(p *Parser, out *bytes.Buffer) {
	out.WriteString("<mspace")
	if m.height != "" {
		out.WriteString(" height='")
		out.WriteString(m.height)
		out.WriteString("'")
	}
	if m.width != "" {
		out.WriteString(" width='")
		out.WriteString(m.width)
		out.WriteString("'")
	}
	out.WriteString(" />")
}

func textOf(m mathNode) string {
	switch m := m.(type) {
	case *mathText:
		return m.s
	case mrow:
		s := ""
		for _, x := range m {
			s += textOf(x)
		}
		return s
	case mathGroup:
		s := ""
		for _, x := range m {
			s += textOf(x)
		}
		return s
	}
	return ""
}

func opMspace(p *Parser) mathNode {
	return &mspace{width: textOf(p.math()), height: textOf(p.math())}
}

/*
	t := p.tex.next()
	if t.eof() {
	}
	if t.syntax("}") {
		break
	}
	p.tex.push(t)
	m := p.math()
	p.push



	switch t.s {
	case `^`:
	case `_`:
	case `\atop`:
	case `\over`:
	case `\mover`:
	case `\munder`:
	case `\mphantom`:
	case `\mi`:
	case `\mo`:
	case `\mattr`:
	case `\mtable`:
	case `\mtr`:
	case `\mtd`:
	case `\mrow`:


		// TODO TOO GENERAL
		// TODO \mathml{kind}{attrs}{body}
		// TODO \def\mathml#1#2#3{\html{\htmltag{#1}{#2}#3\htmltag{/#1}{}}}
		// TODO \def\sqrt#1{\mathml{msqrt}{}{#1}}
		// TODO \def\overline#1{\mathml{mover}{}{\mwrap{#1}\html{<mo>\_</mo>}}}
		// TODO \def\underline#1{\mathml{munder}{}{\mwrap{#1}\html{<mo>\_</mo>}}}
		// TODO \def\phantom#1{\mathml{
	}
*/

func (p *Parser) ToHTML(text string) (mathml string, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	p.tex.push(tokens(pos{"input", 1}, text)...)
	m := p.maths()
	var out bytes.Buffer
	m.mathml(p, &out)
	return out.String(), nil
}

func ToHTML(text string) (string, error) {
	return NewParser().ToHTML(text)
}

var (
	serifBold       = strings.NewReplacer("A", "𝐀", "B", "𝐁", "C", "𝐂", "D", "𝐃", "E", "𝐄", "F", "𝐅", "G", "𝐆", "H", "𝐇", "I", "𝐈", "J", "𝐉", "K", "𝐊", "L", "𝐋", "M", "𝐌", "N", "𝐍", "O", "𝐎", "P", "𝐏", "Q", "𝐐", "R", "𝐑", "S", "𝐒", "T", "𝐓", "U", "𝐔", "V", "𝐕", "W", "𝐖", "X", "𝐗", "Y", "𝐘", "Z", "𝐙", "a", "𝐚", "b", "𝐛", "c", "𝐜", "d", "𝐝", "e", "𝐞", "f", "𝐟", "g", "𝐠", "h", "𝐡", "i", "𝐢", "j", "𝐣", "k", "𝐤", "l", "𝐥", "m", "𝐦", "n", "𝐧", "o", "𝐨", "p", "𝐩", "q", "𝐪", "r", "𝐫", "s", "𝐬", "t", "𝐭", "u", "𝐮", "v", "𝐯", "w", "𝐰", "x", "𝐱", "y", "𝐲", "z", "𝐳", "Α", "𝚨", "Β", "𝚩", "Γ", "𝚪", "Δ", "𝚫", "Ε", "𝚬", "Ζ", "𝚭", "Η", "𝚮", "Θ", "𝚯", "Ι", "𝚰", "Κ", "𝚱", "Λ", "𝚲", "Μ", "𝚳", "Ν", "𝚴", "Ξ", "𝚵", "Ο", "𝚶", "Π", "𝚷", "Ρ", "𝚸", "ϴ", "𝚹", "Σ", "𝚺", "Τ", "𝚻", "Υ", "𝚼", "Φ", "𝚽", "Χ", "𝚾", "Ψ", "𝚿", "Ω", "𝛀", "∇", "𝛁", "α", "𝛂", "β", "𝛃", "γ", "𝛄", "δ", "𝛅", "ε", "𝛆", "ζ", "𝛇", "η", "𝛈", "θ", "𝛉", "ι", "𝛊", "κ", "𝛋", "λ", "𝛌", "μ", "𝛍", "ν", "𝛎", "ξ", "𝛏", "ο", "𝛐", "π", "𝛑", "ρ", "𝛒", "ς", "𝛓", "σ", "𝛔", "τ", "𝛕", "υ", "𝛖", "φ", "𝛗", "χ", "𝛘", "ψ", "𝛙", "ω", "𝛚", "∂", "𝛛", "ϵ", "𝛜", "ϑ", "𝛝", "ϰ", "𝛞", "ϕ", "𝛟", "ϱ", "𝛠", "ϖ", "𝛡", "0", "𝟎", "1", "𝟏", "2", "𝟐", "3", "𝟑", "4", "𝟒", "5", "𝟓", "6", "𝟔", "7", "𝟕", "8", "𝟖", "9", "𝟗")
	serifItalic     = strings.NewReplacer("A", "𝐴", "B", "𝐵", "C", "𝐶", "D", "𝐷", "E", "𝐸", "F", "𝐹", "G", "𝐺", "H", "𝐻", "I", "𝐼", "J", "𝐽", "K", "𝐾", "L", "𝐿", "M", "𝑀", "N", "𝑁", "O", "𝑂", "P", "𝑃", "Q", "𝑄", "R", "𝑅", "S", "𝑆", "T", "𝑇", "U", "𝑈", "V", "𝑉", "W", "𝑊", "X", "𝑋", "Y", "𝑌", "Z", "𝑍", "a", "𝑎", "b", "𝑏", "c", "𝑐", "d", "𝑑", "e", "𝑒", "f", "𝑓", "g", "𝑔", "h", "ℎ", "i", "𝑖", "j", "𝑗", "k", "𝑘", "l", "𝑙", "m", "𝑚", "n", "𝑛", "o", "𝑜", "p", "𝑝", "q", "𝑞", "r", "𝑟", "s", "𝑠", "t", "𝑡", "u", "𝑢", "v", "𝑣", "w", "𝑤", "x", "𝑥", "y", "𝑦", "z", "𝑧", "Α", "𝛢", "Β", "𝛣", "Γ", "𝛤", "Δ", "𝛥", "Ε", "𝛦", "Ζ", "𝛧", "Η", "𝛨", "Θ", "𝛩", "Ι", "𝛪", "Κ", "𝛫", "Λ", "𝛬", "Μ", "𝛭", "Ν", "𝛮", "Ξ", "𝛯", "Ο", "𝛰", "Π", "𝛱", "Ρ", "𝛲", "ϴ", "𝛳", "Σ", "𝛴", "Τ", "𝛵", "Υ", "𝛶", "Φ", "𝛷", "Χ", "𝛸", "Ψ", "𝛹", "Ω", "𝛺", "∇", "𝛻", "α", "𝛼", "β", "𝛽", "γ", "𝛾", "δ", "𝛿", "ε", "𝜀", "ζ", "𝜁", "η", "𝜂", "θ", "𝜃", "ι", "𝜄", "κ", "𝜅", "λ", "𝜆", "μ", "𝜇", "ν", "𝜈", "ξ", "𝜉", "ο", "𝜊", "π", "𝜋", "ρ", "𝜌", "ς", "𝜍", "σ", "𝜎", "τ", "𝜏", "υ", "𝜐", "φ", "𝜑", "χ", "𝜒", "ψ", "𝜓", "ω", "𝜔", "∂", "𝜕", "ϵ", "𝜖", "ϑ", "𝜗", "ϰ", "𝜘", "ϕ", "𝜙", "ϱ", "𝜚", "ϖ", "𝜛")
	serifBoldItalic = strings.NewReplacer("A", "𝑨", "B", "𝑩", "C", "𝑪", "D", "𝑫", "E", "𝑬", "F", "𝑭", "G", "𝑮", "H", "𝑯", "I", "𝑰", "J", "𝑱", "K", "𝑲", "L", "𝑳", "M", "𝑴", "N", "𝑵", "O", "𝑶", "P", "𝑷", "Q", "𝑸", "R", "𝑹", "S", "𝑺", "T", "𝑻", "U", "𝑼", "V", "𝑽", "W", "𝑾", "X", "𝑿", "Y", "𝒀", "Z", "𝒁", "a", "𝒂", "b", "𝒃", "c", "𝒄", "d", "𝒅", "e", "𝒆", "f", "𝒇", "g", "𝒈", "h", "𝒉", "i", "𝒊", "j", "𝒋", "k", "𝒌", "l", "𝒍", "m", "𝒎", "n", "𝒏", "o", "𝒐", "p", "𝒑", "q", "𝒒", "r", "𝒓", "s", "𝒔", "t", "𝒕", "u", "𝒖", "v", "𝒗", "w", "𝒘", "x", "𝒙", "y", "𝒚", "z", "𝒛", "Α", "𝜜", "Β", "𝜝", "Γ", "𝜞", "Δ", "𝜟", "Ε", "𝜠", "Ζ", "𝜡", "Η", "𝜢", "Θ", "𝜣", "Ι", "𝜤", "Κ", "𝜥", "Λ", "𝜦", "Μ", "𝜧", "Ν", "𝜨", "Ξ", "𝜩", "Ο", "𝜪", "Π", "𝜫", "Ρ", "𝜬", "ϴ", "𝜭", "Σ", "𝜮", "Τ", "𝜯", "Υ", "𝜰", "Φ", "𝜱", "Χ", "𝜲", "Ψ", "𝜳", "Ω", "𝜴", "∇", "𝜵", "α", "𝜶", "β", "𝜷", "γ", "𝜸", "δ", "𝜹", "ε", "𝜺", "ζ", "𝜻", "η", "𝜼", "θ", "𝜽", "ι", "𝜾", "κ", "𝜿", "λ", "𝝀", "μ", "𝝁", "ν", "𝝂", "ξ", "𝝃", "ο", "𝝄", "π", "𝝅", "ρ", "𝝆", "ς", "𝝇", "σ", "𝝈", "τ", "𝝉", "υ", "𝝊", "φ", "𝝋", "χ", "𝝌", "ψ", "𝝍", "ω", "𝝎", "∂", "𝝏", "ϵ", "𝝐", "ϑ", "𝝑", "ϰ", "𝝒", "ϕ", "𝝓", "ϱ", "𝝔", "ϖ", "𝝕")
	sansNormal      = strings.NewReplacer("A", "𝖠", "B", "𝖡", "C", "𝖢", "D", "𝖣", "E", "𝖤", "F", "𝖥", "G", "𝖦", "H", "𝖧", "I", "𝖨", "J", "𝖩", "K", "𝖪", "L", "𝖫", "M", "𝖬", "N", "𝖭", "O", "𝖮", "P", "𝖯", "Q", "𝖰", "R", "𝖱", "S", "𝖲", "T", "𝖳", "U", "𝖴", "V", "𝖵", "W", "𝖶", "X", "𝖷", "Y", "𝖸", "Z", "𝖹", "a", "𝖺", "b", "𝖻", "c", "𝖼", "d", "𝖽", "e", "𝖾", "f", "𝖿", "g", "𝗀", "h", "𝗁", "i", "𝗂", "j", "𝗃", "k", "𝗄", "l", "𝗅", "m", "𝗆", "n", "𝗇", "o", "𝗈", "p", "𝗉", "q", "𝗊", "r", "𝗋", "s", "𝗌", "t", "𝗍", "u", "𝗎", "v", "𝗏", "w", "𝗐", "x", "𝗑", "y", "𝗒", "z", "𝗓", "0", "𝟢", "1", "𝟣", "2", "𝟤", "3", "𝟥", "4", "𝟦", "5", "𝟧", "6", "𝟨", "7", "𝟩", "8", "𝟪", "9", "𝟫")
	sansBold        = strings.NewReplacer("A", "𝗔", "B", "𝗕", "C", "𝗖", "D", "𝗗", "E", "𝗘", "F", "𝗙", "G", "𝗚", "H", "𝗛", "I", "𝗜", "J", "𝗝", "K", "𝗞", "L", "𝗟", "M", "𝗠", "N", "𝗡", "O", "𝗢", "P", "𝗣", "Q", "𝗤", "R", "𝗥", "S", "𝗦", "T", "𝗧", "U", "𝗨", "V", "𝗩", "W", "𝗪", "X", "𝗫", "Y", "𝗬", "Z", "𝗭", "a", "𝗮", "b", "𝗯", "c", "𝗰", "d", "𝗱", "e", "𝗲", "f", "𝗳", "g", "𝗴", "h", "𝗵", "i", "𝗶", "j", "𝗷", "k", "𝗸", "l", "𝗹", "m", "𝗺", "n", "𝗻", "o", "𝗼", "p", "𝗽", "q", "𝗾", "r", "𝗿", "s", "𝘀", "t", "𝘁", "u", "𝘂", "v", "𝘃", "w", "𝘄", "x", "𝘅", "y", "𝘆", "z", "𝘇", "Α", "𝝖", "Β", "𝝗", "Γ", "𝝘", "Δ", "𝝙", "Ε", "𝝚", "Ζ", "𝝛", "Η", "𝝜", "Θ", "𝝝", "Ι", "𝝞", "Κ", "𝝟", "Λ", "𝝠", "Μ", "𝝡", "Ν", "𝝢", "Ξ", "𝝣", "Ο", "𝝤", "Π", "𝝥", "Ρ", "𝝦", "ϴ", "𝝧", "Σ", "𝝨", "Τ", "𝝩", "Υ", "𝝪", "Φ", "𝝫", "Χ", "𝝬", "Ψ", "𝝭", "Ω", "𝝮", "∇", "𝝯", "α", "𝝰", "β", "𝝱", "γ", "𝝲", "δ", "𝝳", "ε", "𝝴", "ζ", "𝝵", "η", "𝝶", "θ", "𝝷", "ι", "𝝸", "κ", "𝝹", "λ", "𝝺", "μ", "𝝻", "ν", "𝝼", "ξ", "𝝽", "ο", "𝝾", "π", "𝝿", "ρ", "𝞀", "ς", "𝞁", "σ", "𝞂", "τ", "𝞃", "υ", "𝞄", "φ", "𝞅", "χ", "𝞆", "ψ", "𝞇", "ω", "𝞈", "∂", "𝞉", "ϵ", "𝞊", "ϑ", "𝞋", "ϰ", "𝞌", "ϕ", "𝞍", "ϱ", "𝞎", "ϖ", "𝞏", "0", "𝟬", "1", "𝟭", "2", "𝟮", "3", "𝟯", "4", "𝟰", "5", "𝟱", "6", "𝟲", "7", "𝟳", "8", "𝟴", "9", "𝟵")
	sansItalic      = strings.NewReplacer("A", "𝘈", "B", "𝘉", "C", "𝘊", "D", "𝘋", "E", "𝘌", "F", "𝘍", "G", "𝘎", "H", "𝘏", "I", "𝘐", "J", "𝘑", "K", "𝘒", "L", "𝘓", "M", "𝘔", "N", "𝘕", "O", "𝘖", "P", "𝘗", "Q", "𝘘", "R", "𝘙", "S", "𝘚", "T", "𝘛", "U", "𝘜", "V", "𝘝", "W", "𝘞", "X", "𝘟", "Y", "𝘠", "Z", "𝘡", "a", "𝘢", "b", "𝘣", "c", "𝘤", "d", "𝘥", "e", "𝘦", "f", "𝘧", "g", "𝘨", "h", "𝘩", "i", "𝘪", "j", "𝘫", "k", "𝘬", "l", "𝘭", "m", "𝘮", "n", "𝘯", "o", "𝘰", "p", "𝘱", "q", "𝘲", "r", "𝘳", "s", "𝘴", "t", "𝘵", "u", "𝘶", "v", "𝘷", "w", "𝘸", "x", "𝘹", "y", "𝘺", "z", "𝘻")
	sansBoldItalic  = strings.NewReplacer("A", "𝘼", "B", "𝘽", "C", "𝘾", "D", "𝘿", "E", "𝙀", "F", "𝙁", "G", "𝙂", "H", "𝙃", "I", "𝙄", "J", "𝙅", "K", "𝙆", "L", "𝙇", "M", "𝙈", "N", "𝙉", "O", "𝙊", "P", "𝙋", "Q", "𝙌", "R", "𝙍", "S", "𝙎", "T", "𝙏", "U", "𝙐", "V", "𝙑", "W", "𝙒", "X", "𝙓", "Y", "𝙔", "Z", "𝙕", "a", "𝙖", "b", "𝙗", "c", "𝙘", "d", "𝙙", "e", "𝙚", "f", "𝙛", "g", "𝙜", "h", "𝙝", "i", "𝙞", "j", "𝙟", "k", "𝙠", "l", "𝙡", "m", "𝙢", "n", "𝙣", "o", "𝙤", "p", "𝙥", "q", "𝙦", "r", "𝙧", "s", "𝙨", "t", "𝙩", "u", "𝙪", "v", "𝙫", "w", "𝙬", "x", "𝙭", "y", "𝙮", "z", "𝙯", "Α", "𝞐", "Β", "𝞑", "Γ", "𝞒", "Δ", "𝞓", "Ε", "𝞔", "Ζ", "𝞕", "Η", "𝞖", "Θ", "𝞗", "Ι", "𝞘", "Κ", "𝞙", "Λ", "𝞚", "Μ", "𝞛", "Ν", "𝞜", "Ξ", "𝞝", "Ο", "𝞞", "Π", "𝞟", "Ρ", "𝞠", "ϴ", "𝞡", "Σ", "𝞢", "Τ", "𝞣", "Υ", "𝞤", "Φ", "𝞥", "Χ", "𝞦", "Ψ", "𝞧", "Ω", "𝞨", "∇", "𝞩", "α", "𝞪", "β", "𝞫", "γ", "𝞬", "δ", "𝞭", "ε", "𝞮", "ζ", "𝞯", "η", "𝞰", "θ", "𝞱", "ι", "𝞲", "κ", "𝞳", "λ", "𝞴", "μ", "𝞵", "ν", "𝞶", "ξ", "𝞷", "ο", "𝞸", "π", "𝞹", "ρ", "𝞺", "ς", "𝞻", "σ", "𝞼", "τ", "𝞽", "υ", "𝞾", "φ", "𝞿", "χ", "𝟀", "ψ", "𝟁", "ω", "𝟂", "∂", "𝟃", "ϵ", "𝟄", "ϑ", "𝟅", "ϰ", "𝟆", "ϕ", "𝟇", "ϱ", "𝟈", "ϖ", "𝟉")
	scriptNormal    = strings.NewReplacer("A", "𝒜", "B", "ℬ", "C", "𝒞", "D", "𝒟", "E", "ℰ", "F", "ℱ", "G", "𝒢", "H", "ℋ", "I", "ℐ", "J", "𝒥", "K", "𝒦", "L", "ℒ", "M", "ℳ", "N", "𝒩", "O", "𝒪", "P", "𝒫", "Q", "𝒬", "R", "ℛ", "S", "𝒮", "T", "𝒯", "U", "𝒰", "V", "𝒱", "W", "𝒲", "X", "𝒳", "Y", "𝒴", "Z", "𝒵", "a", "𝒶", "b", "𝒷", "c", "𝒸", "d", "𝒹", "e", "ℯ", "f", "𝒻", "g", "ℊ", "h", "𝒽", "i", "𝒾", "j", "𝒿", "k", "𝓀", "l", "𝓁", "m", "𝓂", "n", "𝓃", "o", "ℴ", "p", "𝓅", "q", "𝓆", "r", "𝓇", "s", "𝓈", "t", "𝓉", "u", "𝓊", "v", "𝓋", "w", "𝓌", "x", "𝓍", "y", "𝓎", "z", "𝓏")
	scriptBold      = strings.NewReplacer("A", "𝓐", "B", "𝓑", "C", "𝓒", "D", "𝓓", "E", "𝓔", "F", "𝓕", "G", "𝓖", "H", "𝓗", "I", "𝓘", "J", "𝓙", "K", "𝓚", "L", "𝓛", "M", "𝓜", "N", "𝓝", "O", "𝓞", "P", "𝓟", "Q", "𝓠", "R", "𝓡", "S", "𝓢", "T", "𝓣", "U", "𝓤", "V", "𝓥", "W", "𝓦", "X", "𝓧", "Y", "𝓨", "Z", "𝓩", "a", "𝓪", "b", "𝓫", "c", "𝓬", "d", "𝓭", "e", "𝓮", "f", "𝓯", "g", "𝓰", "h", "𝓱", "i", "𝓲", "j", "𝓳", "k", "𝓴", "l", "𝓵", "m", "𝓶", "n", "𝓷", "o", "𝓸", "p", "𝓹", "q", "𝓺", "r", "𝓻", "s", "𝓼", "t", "𝓽", "u", "𝓾", "v", "𝓿", "w", "𝔀", "x", "𝔁", "y", "𝔂", "z", "𝔃")
	frakturNormal   = strings.NewReplacer("A", "𝔄", "B", "𝔅", "C", "ℭ", "D", "𝔇", "E", "𝔈", "F", "𝔉", "G", "𝔊", "H", "ℌ", "I", "ℑ", "J", "𝔍", "K", "𝔎", "L", "𝔏", "M", "𝔐", "N", "𝔑", "O", "𝔒", "P", "𝔓", "Q", "𝔔", "R", "ℜ", "S", "𝔖", "T", "𝔗", "U", "𝔘", "V", "𝔙", "W", "𝔚", "X", "𝔛", "Y", "𝔜", "Z", "ℨ", "a", "𝔞", "b", "𝔟", "c", "𝔠", "d", "𝔡", "e", "𝔢", "f", "𝔣", "g", "𝔤", "h", "𝔥", "i", "𝔦", "j", "𝔧", "k", "𝔨", "l", "𝔩", "m", "𝔪", "n", "𝔫", "o", "𝔬", "p", "𝔭", "q", "𝔮", "r", "𝔯", "s", "𝔰", "t", "𝔱", "u", "𝔲", "v", "𝔳", "w", "𝔴", "x", "𝔵", "y", "𝔶", "z", "𝔷")
	frakturBold     = strings.NewReplacer("A", "𝕬", "B", "𝕭", "C", "𝕮", "D", "𝕯", "E", "𝕰", "F", "𝕱", "G", "𝕲", "H", "𝕳", "I", "𝕴", "J", "𝕵", "K", "𝕶", "L", "𝕷", "M", "𝕸", "N", "𝕹", "O", "𝕺", "P", "𝕻", "Q", "𝕼", "R", "𝕽", "S", "𝕾", "T", "𝕿", "U", "𝖀", "V", "𝖁", "W", "𝖂", "X", "𝖃", "Y", "𝖄", "Z", "𝖅", "a", "𝖆", "b", "𝖇", "c", "𝖈", "d", "𝖉", "e", "𝖊", "f", "𝖋", "g", "𝖌", "h", "𝖍", "i", "𝖎", "j", "𝖏", "k", "𝖐", "l", "𝖑", "m", "𝖒", "n", "𝖓", "o", "𝖔", "p", "𝖕", "q", "𝖖", "r", "𝖗", "s", "𝖘", "t", "𝖙", "u", "𝖚", "v", "𝖛", "w", "𝖜", "x", "𝖝", "y", "𝖞", "z", "𝖟")
	monoNormal      = strings.NewReplacer("A", "𝙰", "B", "𝙱", "C", "𝙲", "D", "𝙳", "E", "𝙴", "F", "𝙵", "G", "𝙶", "H", "𝙷", "I", "𝙸", "J", "𝙹", "K", "𝙺", "L", "𝙻", "M", "𝙼", "N", "𝙽", "O", "𝙾", "P", "𝙿", "Q", "𝚀", "R", "𝚁", "S", "𝚂", "T", "𝚃", "U", "𝚄", "V", "𝚅", "W", "𝚆", "X", "𝚇", "Y", "𝚈", "Z", "𝚉", "a", "𝚊", "b", "𝚋", "c", "𝚌", "d", "𝚍", "e", "𝚎", "f", "𝚏", "g", "𝚐", "h", "𝚑", "i", "𝚒", "j", "𝚓", "k", "𝚔", "l", "𝚕", "m", "𝚖", "n", "𝚗", "o", "𝚘", "p", "𝚙", "q", "𝚚", "r", "𝚛", "s", "𝚜", "t", "𝚝", "u", "𝚞", "v", "𝚟", "w", "𝚠", "x", "𝚡", "y", "𝚢", "z", "𝚣", "0", "𝟶", "1", "𝟷", "2", "𝟸", "3", "𝟹", "4", "𝟺", "5", "𝟻", "6", "𝟼", "7", "𝟽", "8", "𝟾", "9", "𝟿")
	doubleBold      = strings.NewReplacer("A", "𝔸", "B", "𝔹", "C", "ℂ", "D", "𝔻", "E", "𝔼", "F", "𝔽", "G", "𝔾", "H", "ℍ", "I", "𝕀", "J", "𝕁", "K", "𝕂", "L", "𝕃", "M", "𝕄", "N", "ℕ", "O", "𝕆", "P", "ℙ", "Q", "ℚ", "R", "ℝ", "S", "𝕊", "T", "𝕋", "U", "𝕌", "V", "𝕍", "W", "𝕎", "X", "𝕏", "Y", "𝕐", "Z", "ℤ", "a", "𝕒", "b", "𝕓", "c", "𝕔", "d", "𝕕", "e", "𝕖", "f", "𝕗", "g", "𝕘", "h", "𝕙", "i", "𝕚", "j", "𝕛", "k", "𝕜", "l", "𝕝", "m", "𝕞", "n", "𝕟", "o", "𝕠", "p", "𝕡", "q", "𝕢", "r", "𝕣", "s", "𝕤", "t", "𝕥", "u", "𝕦", "v", "𝕧", "w", "𝕨", "x", "𝕩", "y", "𝕪", "z", "𝕫", "0", "𝟘", "1", "𝟙", "2", "𝟚", "3", "𝟛", "4", "𝟜", "5", "𝟝", "6", "𝟞", "7", "𝟟", "8", "𝟠", "9", "𝟡")
)
