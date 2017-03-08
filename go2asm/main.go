// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Go2asm converts the Go compiler's -S output to equivalent assembler source files.
//
// Usage:
//
//	go2asm [-s symregexp] [file]
//
// Go2asm reads the compiler's -S output from file (default standard input),
// converting it to equivalent assembler input. If the -s option is present,
// go2asm only converts symbols with names matching the regular expression.
//
// Example
//
// Extract the assembly for a test program:
//
//	$ cat /tmp/x.go
//	package p
//
//	func f(x int) (y int) {
//		return x / 10
//	}
//	$ go tool compile -S /tmp/x.go | go2asm
//	#include "funcdata.h"
//
//	TEXT ·f(SB), $0-16 // /tmp/x.go:3
//		NO_LOCAL_POINTERS
//		// FUNCDATA $0, gclocals·f207267fbf96a0178e8758c6e3e0ce28(SB) (args)
//		// FUNCDATA $1, gclocals·33cdeccccebe80329f1fdbee7f5874cb(SB) (no locals)
//		MOVQ       $-3689348814741910323, AX  // x.go:4
//		MOVQ       x+0(FP), CX
//		IMULQ      CX
//		ADDQ       CX, DX
//		SARQ       $3, DX
//		SARQ       $63, CX
//		SUBQ       CX, DX
//		MOVQ       DX, y+8(FP)
//		RET
//	$
//
// Extract the assembly for the function math.IsInf:
//
//	$ go build -a -v -gcflags -S math 2>&1 | go2asm -s math.IsInf
//	TEXT math·IsInf(SB), $0-24 // /Users/rsc/go/src/math/bits.go:43
//		NO_LOCAL_POINTERS
//		// FUNCDATA $0, gclocals·54241e171da8af6ae173d69da0236748(SB) (args)
//		// FUNCDATA $1, gclocals·33cdeccccebe80329f1fdbee7f5874cb(SB) (no locals)
//		MOVQ       sign+8(FP), AX  // bits.go:48
//		TESTQ      AX, AX
//		JLT        pc66
//		MOVSD      f+0(FP), X0
//		MOVSD      $(1.7976931348623157e+308), X1
//		UCOMISD    X1, X0
//		JLS        pc40
//		MOVL       $1, AX
//	pc35:
//		MOVB       AL, _r2+16(FP)
//		RET
//	pc40:
//		TESTQ      AX, AX
//	pc43:
//		JGT        pc62
//		MOVSD      $(-1.7976931348623157e+308), X1
//		UCOMISD    X0, X1
//		SETHI      AL
//		JMP        pc35
//	pc62:
//		MOVL       $0, AX
//		JMP        pc35
//	pc66:
//		MOVSD      f+0(FP), X0
//		JMP        pc43
//	$
//
// Bugs
//
// Go2asm only handles amd64 assembler.
//
// Data symbols are not implemented.
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	startTextRE = regexp.MustCompile(`^(""\.[^ ]+) t=([^ ]+) size=([^ ]+) (?:value=[^ ]+ )?args=([^ ]+) locals=([^ ]+)$`)
	startDataRE = regexp.MustCompile(`^([^ ]+) t=([^ ]+) size=([^ ]+)$`)
	instRE      = regexp.MustCompile(`^\t(0x[0-9a-f]+) 0*(0|[1-9][0-9]*) \(([^\t]+:[0-9]+)\)\t([A-Z0-9].*)$`)
)

var (
	input string
	pkg   string
	sym   string

	wordSize = 8

	symRE   = regexp.MustCompile(``)
	symFlag = flag.String("s", "", "print only symbols matching `symregexp`")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go2asm [-s symregexp] [file]\n")
	os.Exit(2)
}

func main() {
	log.SetPrefix("go2asm: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() > 1 {
		usage()
	}

	if *symFlag != "" {
		re, err := regexp.Compile(*symFlag)
		if err != nil {
			log.Fatal("invalid -s regexp: %s", err)
		}
		symRE = re
	}

	var data []byte
	var err error
	if flag.NArg() == 0 {
		data, err = ioutil.ReadAll(os.Stdin)
		input = "<stdin>"
	} else {
		input = flag.Arg(0)
		data, err = ioutil.ReadFile(flag.Arg(0))
	}
	if err != nil {
		log.Fatal(err)
	}

	var (
		mode string
		text []Inst
	)

	flush := func() {
		if mode == "text" {
			asmText(text)
		}
		mode = ""
		text = nil
		sym = ""
	}

	for lineno, line := range strings.Split(string(data), "\n") {
		lineno++
		if !strings.HasPrefix(line, "\t") {
			flush()
		}
		if strings.HasPrefix(line, "# ") && !strings.Contains(line[2:], " ") {
			pkg = line[2:]
		}
		if m := startTextRE.FindStringSubmatch(line); m != nil {
			sym = m[1]
			if !symRE.MatchString(pkg + "." + sym[3:]) {
				continue
			}
			mode = "text"
			continue
		}
		if m := startDataRE.FindStringSubmatch(line); m != nil {
			sym = m[1]
			if !symRE.MatchString(pkg + "." + sym[3:]) {
				continue
			}
			mode = "data"
			continue
		}
		if mode == "text" {
			if m := instRE.FindStringSubmatch(line); m != nil {
				if len(text) == 0 && !strings.HasPrefix(m[4], "TEXT\t"+sym+"(SB),") {
					warn(lineno, "did not find TEXT at start of %s: %s", sym, m[4])
				}
				text = append(text, Inst{Lineno: lineno, PC: m[2], FileLine: m[3], Asm: m[4]})
				continue
			}
		}
	}
	flush()
}

func warn(lineno int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", input, lineno, fmt.Sprintf(format, args...))
}

type Inst struct {
	Lineno   int    // line number in our input (compiler -S output)
	PC       string // decimal PC (second column of -S output)
	FileLine string // file:line (third column of -S output)
	Asm      string // assembly instruction
}

var haveFuncdataH = false

var (
	textRE        = regexp.MustCompile(`TEXT.*\(SB\), \$([0-9]+)-([0-9]+)$`)
	flt64RE       = regexp.MustCompile(`\$f64\.[0-9a-f]{16}\(SB\)`)
	spRE          = regexp.MustCompile(`\+[0-9]+\((FP|SP)\)`)
	stackPkgRE    = regexp.MustCompile(`""\.([^ ,\t]+)\+[0-9]+\((SP|FP)\)`)
	tildeResultRE = regexp.MustCompile(`[.~][a-z0-9_]+\+[0-9]+\((SP|FP)\)`)
)

func asmText(text []Inst) {
	var buf bytes.Buffer

	var (
		noLocalPointers bool
		locals          int
		args            int
		inStackPrologue bool
		cutBP           bool
	)

	pkgPrefix := strings.Replace(strings.Replace(pathtoprefix(pkg)+".", "/", "∕", -1), ".", "·", -1)

	for i := range text {
		inst := &text[i]

		if strings.HasPrefix(inst.Asm, "MOVQ\t(TLS)") {
			inst.Asm = "// " + inst.Asm + " (stack growth prologue)"
			inStackPrologue = true
			continue
		}
		if strings.HasPrefix(inst.Asm, "SUBQ\t$") && strings.HasSuffix(inst.Asm, ", SP") {
			inst.Asm = "// " + inst.Asm
			inStackPrologue = false
		}
		if strings.HasPrefix(inst.Asm, "ADDQ\t$") && strings.HasSuffix(inst.Asm, ", SP") { // SP rewind before RET
			inst.Asm = "// " + inst.Asm + " (SP restore)"
		}
		if inStackPrologue {
			inst.Asm = "// " + inst.Asm
			continue
		}
		if strings.HasPrefix(inst.Asm, "MOVQ\tBP, ") && strings.HasSuffix(inst.Asm, "(SP)") { // BP save at beginning of function
			inst.Asm = "// " + inst.Asm + " (BP save)"
			cutBP = true
		}
		if strings.HasPrefix(inst.Asm, "LEAQ\t") && strings.HasSuffix(inst.Asm, "(SP), BP") {
			inst.Asm = "// " + inst.Asm + " (BP init)"
		}
		if strings.HasPrefix(inst.Asm, "MOVQ\t") && strings.HasSuffix(inst.Asm, "(SP), BP") { // BP fixup before RET
			inst.Asm = "// " + inst.Asm + " (BP restore)"
		}
		if m := textRE.FindStringSubmatch(inst.Asm); m != nil {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				warn(inst.Lineno, "invalid locals size: %s", inst.Asm)
			}
			locals = n
			n, err = strconv.Atoi(m[2])
			if err != nil {
				warn(inst.Lineno, "invalid args size: %s", inst.Asm)
			}
			args = n
		}

		// Comment out no-op FUNCDATAs.
		if strings.HasPrefix(inst.Asm, "FUNCDATA\t$0,") { // args pointer map
			inst.Asm = "// " + inst.Asm + " (args)"
		}
		if strings.HasPrefix(inst.Asm, "FUNCDATA\t$1,") { // locals pointer map
			if inst.Asm == "FUNCDATA\t$1, gclocals·33cdeccccebe80329f1fdbee7f5874cb(SB)" {
				inst.Asm = "// " + inst.Asm + " (no locals)"
				noLocalPointers = true
			} else {
				inst.Asm += " (locals)"
			}
		}

		if strings.HasPrefix(inst.Asm, "//") {
			continue
		}

		// Rewrite $f64.0xbits into floating-point constant.
		// TODO: Also $f32.
		inst.Asm = flt64RE.ReplaceAllStringFunc(inst.Asm, func(name string) string {
			v, err := strconv.ParseUint(name[len("$f64."):len(name)-len("(SB)")], 16, 64)
			if err != nil {
				warn(inst.Lineno, "invalid $f64 reference: %s", inst.Asm)
				return name
			}
			f := math.Float64frombits(v)
			g := fmt.Sprintf("%g", f)
			if !strings.Contains(g, "e") && !strings.Contains(g, ".") {
				g += ".0" // $(1) is not float; need $(1.0).
			}
			return "$(" + g + ")"
		})

		// In local variable names, drop "". prefix (for early versions of Go).
		inst.Asm = stackPkgRE.ReplaceAllStringFunc(inst.Asm, func(name string) string {
			return name[len(`"".`):]
		})

		// Replace ~r1 with _r1.
		inst.Asm = tildeResultRE.ReplaceAllStringFunc(inst.Asm, func(name string) string {
			return "_" + name[1:]
		})

		// In global variable names, replace "". with assembler prefix (e.g., "math·").
		inst.Asm = strings.Replace(inst.Asm, `"".`, pkgPrefix, -1)

		// Rewrite x+N(SP) and x+N(FP) to be in assembler form.
		// By default the compiler prints N = the exact offset from the real SP.
		// But the assembler expects the offset from the virtual SP or virtual FP.
		inst.Asm = spRE.ReplaceAllStringFunc(inst.Asm, func(name string) string {
			num, suffix := name[len("+"):len(name)-len("(SP)")], name[len(name)-len("(SP)"):]
			off, err := strconv.Atoi(num)
			if err != nil {
				warn(inst.Lineno, "invalid SP reference: %s", inst.Asm)
				return name
			}

			if suffix == "(SP)" {
				off -= locals
				// TODO: BP
				if off >= 0 {
					// Compiler sometimes generates FP refs as SP refs.
					// See golang.org/issue/19458.
					if wordSize <= off && off < args+wordSize {
						off -= wordSize
						suffix = "(FP)"
					} else {
						warn(inst.Lineno, "out-of-bounds SP reference: %s", inst.Asm)
					}
				}
				if cutBP && off < 0 {
					off += wordSize
					if off >= 0 {
						warn(inst.Lineno, "out-of-bounds SP reference: %s", inst.Asm)
					}
				}
			} else { // (FP)
				off -= locals + wordSize
				// TODO: BP
				if off < 0 || off >= args {
					warn(inst.Lineno, "out-of-bounds FP reference: %s", inst.Asm)
				}
			}
			return fmt.Sprintf("%+d%s", off, suffix)
		})
	}

	// Comment out stack growth call at end.
	if len(text) >= 2 && text[len(text)-1].Asm == "JMP\t0" && strings.HasPrefix(text[len(text)-2].Asm, "CALL\truntime.morestack") {
		i := len(text) - 1
		for i >= 0 && (i == len(text)-1 || !strings.HasPrefix(text[i].Asm, "JMP")) && !strings.HasPrefix(text[i].Asm, "RET") {
			text[i].Asm = "// " + text[i].Asm
			i--
		}
		text[i+1].Asm += " (stack growth)"
	}

	// Figure out which instructions need labels for jumps.
	needPC := map[string]bool{}
	for i := range text {
		inst := &text[i]
		if !strings.HasPrefix(inst.Asm, "J") { // jumps of various forms
			continue
		}
		j := strings.Index(inst.Asm, "\t")
		if j < 0 {
			continue
		}
		arg := inst.Asm[j+1:]
		if strings.HasPrefix(arg, "$0, ") || strings.HasPrefix(arg, "$1, ") { // possible prediction hint
			arg = arg[len("$0, "):]
		}
		if _, err := strconv.Atoi(arg); err == nil {
			// is a plain number, assume it's a PC, rewrite 123 to pc123
			needPC[arg] = true
			inst.Asm = inst.Asm[:len(inst.Asm)-len(arg)] + "pc" + arg
		}
	}

	// Replace tab between opcode and args with spaces, to help mini-tabwriter below.
	for i := range text {
		inst := &text[i]
		if i == 0 && strings.HasPrefix(inst.Asm, "TEXT\t") {
			inst.Asm = "TEXT " + inst.Asm[5:]
			continue
		}
		if i := strings.Index(inst.Asm, "\t"); i >= 0 {
			spaces := ""
			if i < 10 {
				spaces = "                            "[:10-i]
			}
			inst.Asm = inst.Asm[:i] + spaces + " " + inst.Asm[i+1:]
		}
	}

	// print assembly
	if !haveFuncdataH && noLocalPointers {
		haveFuncdataH = true
		fmt.Fprintf(&buf, "#include \"funcdata.h\"\n\n")
	}
	where := ""
	for i, inst := range text {
		if i == 0 {
			fmt.Fprintf(&buf, "%s // %s\n", inst.Asm, inst.FileLine)
			if noLocalPointers {
				fmt.Fprintf(&buf, "\tNO_LOCAL_POINTERS\n")
			}
			where = shortFileLine(inst.FileLine)
			continue
		}
		if needPC[inst.PC] {
			fmt.Fprintf(&buf, "pc%s:\n", inst.PC)
			needPC[inst.PC] = false
		}
		fmt.Fprintf(&buf, "\t%s", inst.Asm)
		if w := shortFileLine(inst.FileLine); w != "" && w != where {
			fmt.Fprintf(&buf, "\x01// %s", w)
			where = w
		}
		fmt.Fprintf(&buf, "\n")
	}

	// mini-tabwriter:
	// lines up 2-cell lines but allows 1-cell lines to bleed into second cell.
	// requires second cell to start no farther than maxSpace chars into line.
	const maxSpace = 45
	lines := strings.SplitAfter(buf.String(), "\n")
	max := 0
	for _, line := range lines {
		if i := strings.Index(line, "\x01"); i > max && i < maxSpace {
			max = i
		}
	}
	max++
	spaces := strings.Repeat(" ", maxSpace)
	var buf2 bytes.Buffer
	for _, line := range lines {
		i := strings.Index(line, "\x01")
		if i < 0 {
			buf2.WriteString(line)
		} else {
			buf2.WriteString(line[:i])
			n := max - i
			if n < 0 {
				n = 0
			}
			buf2.WriteString(spaces[:n+1])
			buf2.WriteString(line[i+1:])
		}
	}

	os.Stdout.Write(buf2.Bytes())
}

func shortFileLine(f string) string {
	f = f[strings.LastIndex(f, `/`)+1:]
	f = f[strings.LastIndex(f, `\`)+1:]
	return f
}

// From cmd/link/internal/ld/lib.go.
func pathtoprefix(s string) string {
	slash := strings.LastIndex(s, "/")
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= ' ' || i >= slash && c == '.' || c == '%' || c == '"' || c >= 0x7F {
			var buf bytes.Buffer
			for i := 0; i < len(s); i++ {
				c := s[i]
				if c <= ' ' || i >= slash && c == '.' || c == '%' || c == '"' || c >= 0x7F {
					fmt.Fprintf(&buf, "%%%02x", c)
					continue
				}
				buf.WriteByte(c)
			}
			return buf.String()
		}
	}
	return s
}
