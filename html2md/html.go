// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func html2md(file, text string) (string, error) {
	h, err := html.Parse(strings.NewReader(text))
	if err != nil {
		return "", err
	}

	md, err := node2md("", h)
	if err != nil {
		return "", err
	}

	//fmt.Printf("%#v\n", md)

	return mdprint(md), nil
}

func node2md(ctxt string, n *html.Node) (block, error) {
	switch n.Type {
	default:
		return nil, fmt.Errorf("%s: node2md unknown type %v", ctxt, n.Type)
	case html.DocumentNode:
		return block2md(ctxt, n)
	case html.TextNode:
		if strings.TrimSpace(n.Data) == "" {
			return tagBlock(""), nil
		}
		return para{text(n.Data)}, nil
	case html.CommentNode:
		inner := noBlankLines(n.Data)
		if strings.Contains(inner, "\n") {
			inner = "\n" + strings.Trim(inner, "\n") + "\n"
		}
		return tagBlock("<!--" + inner + "-->"), nil
	case html.ElementNode:
		if ctxt != "" {
			ctxt += ">"
		}
		ctxt += n.Data

		switch n.Data {
		default:
			return nil, fmt.Errorf("%s: unhandled node <%s>", ctxt, n.Data)

		case "html", "head", "body":
			return block2md(ctxt, n)

		case "style", "script":
			c := n.FirstChild
			if c != nil && (c.NextSibling != nil || c.Type != html.TextNode) {
				return nil, fmt.Errorf("%s: unexpected child in <%s>", ctxt, n.Data)
			}
			inner := ""
			if c != nil {
				inner = noBlankLines(c.Data)
			}
			if strings.Contains(inner, "\n") {
				inner = "\n" + strings.Trim(inner, "\n") + "\n"
			}
			return tagBlock(tagText(n) + inner + "</" + n.Data + ">"), nil

		case "h1", "h2", "h3", "h4", "h5", "h6":
			inner, err := inline2md(ctxt, n)
			if err != nil {
				return nil, err
			}
			h := heading{level: int(n.Data[1] - '0'), inner: inner}
			if id := attr(n, "id"); id != "" {
				h.id = id
			}
			return h, nil

		case "p":
			inner, err := inline2md(ctxt, n)
			if err != nil {
				return nil, err
			}
			return para(inner), nil

		case "pre":
			c := n.FirstChild
			if c != nil && c.NextSibling == nil && c.Type == html.TextNode {
				return pre(c.Data), nil
			}
			return tagBlock(printHTML(n)), nil

		case "textarea", "select":
			return tagBlock(printHTML(n)), nil

		case "blockquote":
			inner, err := block2md(ctxt, n)
			if err != nil {
				return nil, err
			}
			return quote(inner), nil

		case "ul", "ol":
			var l list
			if n.Data == "ol" {
				l.num = 1
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
					continue
				}
				if c.Type != html.ElementNode || c.Data != "li" {
					return nil, fmt.Errorf("%s: unexpected %s", ctxt, printHTML(c))
				}
				item, err := block2md(ctxt+">li", c)
				if err != nil {
					return nil, err
				}
				l.items = append(l.items, item)
			}
			return l, nil

		case "dl":
			var l defns
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
					continue
				}
				if c.Type != html.ElementNode || c.Data != "dt" && c.Data != "dd" || c.Data == "dd" && len(l) == 0 {
					return nil, fmt.Errorf("%s: unexpected %d %q", ctxt, c.Type, c.Data)
				}
				if c.Data == "dt" {
					inner, err := inline2md(ctxt+">dt", c)
					if err != nil {
						return nil, err
					}
					l = append(l, defn{dt: inner})
					continue
				}
				inner, err := block2md(ctxt+">dd", c)
				if err != nil {
					return nil, err
				}
				l[len(l)-1].dd = append(l[len(l)-1].dd, inner...)
			}
			return l, nil

		case "aside", "div", "section", "iframe":
			// TODO preserve all attributes
			inner, err := block2md(ctxt, n)
			if err != nil {
				return nil, err
			}
			b := blocks{tagBlock(tagText(n))}
			b = append(b, inner...)
			b = append(b, tagBlock("</"+n.Data+">"))
			return b, nil

		case "table":
			return tagBlock(noBlankLines(printHTML(n))), nil
		}
	}
}

func set(s string) map[string]bool {
	m := make(map[string]bool)
	for _, k := range strings.Fields(s) {
		m[k] = true
	}
	return m
}

var blockTag = set(`
	aside
	blockquote
	body
	dd
	div
	dl
	dt
	h1 h2 h3 h4 h5 h6
	head
	html
	iframe
	ol ul li
	p
	pre
	script
	section
	select
	style
	table
	textarea
`)

var inlineTag = set(`
	a
	b
	br
	button
	code
	dfn
	em
	i
	img
	small
	span
	strong
	sub
	sup
	var
`)

func block2md(ctxt string, n *html.Node) (blocks, error) {
	var out blocks
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		switch {
		default:
			return nil, fmt.Errorf("%s: block2md unknown type %v %q", ctxt, c.Type, c.Data)

		case c.Type == html.TextNode || c.Type == html.ElementNode && inlineTag[c.Data]:
			inl, rest, err := collectInline(ctxt, c)
			if err != nil {
				return nil, err
			}
			for len(inl) > 0 {
				t, ok := inl[0].(text)
				if !ok || strings.TrimSpace(string(t)) != "" {
					break
				}
				inl = inl[1:]
			}
			if len(inl) > 0 {
				out = append(out, para{inl})
			}
			if rest == c {
				return nil, fmt.Errorf("%s>%s: stuck in collectPara", ctxt, c.Data)
			}
			next = rest

		case c.Type == html.CommentNode || c.Type == html.ElementNode && blockTag[c.Data]:
			b, err := node2md(ctxt, c)
			if err != nil {
				return nil, err
			}
			if bb, ok := b.(blocks); ok {
				out = append(out, bb...)
			} else if b != tagBlock("") {
				out = append(out, b)
			}

		case c.Type == html.ElementNode:
			return nil, fmt.Errorf("%s: unknown tag %s", ctxt, c.Data)
		}
	}
	return out, nil
}

func inline2md(ctxt string, n *html.Node) (inlines, error) {
	list, rest, err := collectInline(ctxt, n.FirstChild)
	if err != nil {
		return nil, err
	}
	if rest != nil {
		if rest.Type == html.ElementNode {
			return nil, fmt.Errorf("%s: non-inline <%s>", ctxt, rest.Data)
		}
		return nil, fmt.Errorf("%s: non-inline %v %q", ctxt, rest.Type, rest.Data)
	}
	return list, nil
}

func collectInline(ctxt string, c *html.Node) (inlines, *html.Node, error) {
	var out inlines
	for ; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			out = append(out, text(c.Data))
			continue
		}
		if c.Type == html.CommentNode {
			inner := noBlankLines(c.Data)
			if strings.Contains(inner, "\n") {
				inner = "\n" + strings.Trim(inner, "\n") + "\n"
			}
			out = append(out, tag("<!--"+inner+"-->"))
			continue
		}
		if c.Type != html.ElementNode || !inlineTag[c.Data] {
			break
		}
		switch c.Data {
		default:
			return nil, nil, fmt.Errorf("%s>%s: unhandled inline tag", ctxt, c.Data)

		case "a":
			if extraAttr(c, "href") {
				goto SpanTag
			}
			inner, err := inline2md(ctxt, c)
			if err != nil {
				return nil, nil, err
			}
			url := attr(c, "href")
			if strings.HasPrefix(url, "//") {
				url = "https:" + url
			}
			out = append(out, link{url, inner})

		case "br":
			if extraAttr(c) {
				goto SpanTag
			}
			out = append(out, hardBreak{})

		case "b", "strong":
			if extraAttr(c) {
				goto SpanTag
			}
			inner, err := inline2md(ctxt, c)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, strong{inner})

		case "i", "em", "dfn", "var":
			if extraAttr(c) {
				goto SpanTag
			}
			inner, err := inline2md(ctxt, c)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, emph{inner})

		case "code":
			if extraAttr(c) {
				goto SpanTag
			}
			inner, err := inline2md(ctxt, c)
			if err != nil {
				return nil, nil, err
			}
			if len(inner) == 1 {
				if t, ok := inner[0].(text); ok {
					out = append(out, code(t))
					continue
				}
				// turn <code><a>text</a></code>
				// into <a><code>text</code></a>
				if a, ok := inner[0].(link); ok {
					if len(a.inner) == 1 {
						if t, ok := a.inner[0].(text); ok {
							a.inner[0] = code(t)
							out = append(out, a)
							continue
						}
					}
				}
			}
			out = append(out, tag(tagText(c)))
			out = append(out, inner...)
			out = append(out, tag("</code>"))

		case "small", "span", "sup", "sub", "button":
			goto SpanTag

		case "img":
			out = append(out, tag(tagText(c)))
		}
		continue

	SpanTag:
		inner, err := inline2md(ctxt, c)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, tag(tagText(c)))
		out = append(out, inner...)
		out = append(out, tag("</"+c.Data+">"))
	}
	return out, c, nil
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func extraAttr(n *html.Node, known ...string) bool {
Attrs:
	for _, a := range n.Attr {
		for _, ok := range known {
			if a.Key == ok {
				continue Attrs
			}
		}
		return true
	}
	return false
}

func tagText(n *html.Node) string {
	t := n.Data
	for _, a := range n.Attr {
		if a.Val == "" {
			switch a.Key {
			case "async":
				t += " " + a.Key
				continue
			}
		}
		t += fmt.Sprintf(" %s=%q", a.Key, html.EscapeString(a.Val))
	}
	return "<" + t + ">"
}

func noBlankLines(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line != "" {
			out = append(out, line)
		}
	}
	if strings.HasSuffix(s, "\n") && len(out) > 0 {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

func printHTML(n *html.Node) string {
	var buf bytes.Buffer
	printTo(&buf, n)
	return buf.String()
}

func printTo(buf *bytes.Buffer, n *html.Node) {
	switch n.Type {
	case html.CommentNode:
		fmt.Fprintf(buf, "<!--%s-->", html.EscapeString(n.Data))
	case html.TextNode:
		fmt.Fprintf(buf, "%s", html.EscapeString(n.Data))
	case html.ElementNode:
		fmt.Fprintf(buf, "%s", tagText(n))
		if n.Data == "pre" {
			buf.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			printTo(buf, c)
		}
		fmt.Fprintf(buf, "</%s>", n.Data)
	}
}
