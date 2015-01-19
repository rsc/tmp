// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"rsc.io/tmp/slowgc/liblink"
	"rsc.io/tmp/slowgc/liblink/amd64"
	"rsc.io/tmp/slowgc/liblink/arm"
	"rsc.io/tmp/slowgc/liblink/ppc64"
	"rsc.io/tmp/slowgc/liblink/x86"
)

var arch *liblink.LinkArch
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to this file")
var memprofile = flag.String("memprofile", "", "write memory profile to this file")

func main() {
	t0 := time.Now()
	defer func() {
		log.Printf("%.2fs %s\n", time.Since(t0).Seconds(), strings.Join(os.Args, " "))
	}()

	log.SetPrefix("goliblink: ")
	log.SetFlags(0)
	flag.Parse()
	if flag.NArg() != 3 {
		fmt.Fprintf(os.Stderr, "usage: goliblink infile objfile offset\n")
		os.Exit(2)
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.WriteHeapProfile(f)
	}

	switch build.Default.GOARCH {
	case "amd64":
		arch = &amd64.Linkamd64
	case "amd64p32":
		arch = &amd64.Linkamd64p32
	case "386":
		arch = &x86.Link386
	case "arm":
		arch = &arm.Linkarm
	case "ppc64":
		arch = &ppc64.Linkppc64
	case "ppc64le":
		arch = &ppc64.Linkppc64le
	}

	input()
}

var (
	ctxt   *liblink.Link
	plists = map[int64]*liblink.Plist{}
	syms   = map[int64]*liblink.LSym{}
	progs  = map[int64]*liblink.Prog{}
	hists  = map[int64]*liblink.Hist{}
	undef  = map[interface{}]bool{}
	hashed = map[*liblink.LSym]bool{}
)

func input() {
	args := flag.Args()
	ctxt = liblink.Linknew(arch)
	ctxt.Debugasm = 1
	ctxt.Bso = liblink.Binitw(os.Stdout)
	defer liblink.Bflush(ctxt.Bso)
	ctxt.Diag = log.Fatalf
	f, err := os.Open(args[0])
	if err != nil {
		log.Fatal(err)
	}

	b := bufio.NewReaderSize(f, 1<<20)
	if rdstring(b) != "ctxt" {
		log.Fatal("invalid input - missing ctxt")
	}
	name := rdstring(b)
	if name != ctxt.Arch.Name {
		log.Fatalf("bad arch %s - want %s", name, ctxt.Arch.Name)
	}

	ctxt.Goarm = int32(rdint(b))
	ctxt.Debugasm = int32(rdint(b))
	ctxt.Trimpath = rdstring(b)
	ctxt.Plist = rdplist(b)
	ctxt.Plast = rdplist(b)
	ctxt.Hist = rdhist(b)
	ctxt.Ehist = rdhist(b)
	for {
		i := rdint(b)
		if i < 0 {
			break
		}
		hashed[rdsym(b)] = true
	}
	last := "ctxt"

Loop:
	for {
		s := rdstring(b)
		switch s {
		default:
			log.Fatalf("unexpected input after %s: %v", s, last)
		case "end":
			break Loop
		case "plist":
			readplist(b, rdplist(b))
		case "sym":
			readsym(b, rdsym(b))
		case "prog":
			readprog(b, rdprog(b))
		case "hist":
			readhist(b, rdhist(b))
		}
		last = s
	}

	if len(undef) > 0 {
		panic("missing definitions")
	}

	ctxt.Hash = make(map[liblink.NameVers]*liblink.LSym)
	for s := range hashed {
		if s == nil {
			continue
		}
		ctxt.Hash[liblink.NameVers{s.Name, int(s.Version)}] = s
	}

	var buf bytes.Buffer
	obuf := liblink.Binitw(&buf)
	liblink.Writeobjdirect(ctxt, obuf)
	liblink.Bflush(obuf)

	data, err := ioutil.ReadFile(args[1])
	if err != nil {
		log.Fatal(err)
	}

	offset, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("bad offset: %v", err)
	}
	if offset > len(data) {
		log.Fatalf("offset too large: %v > %v", offset, len(data))
	}

	old := data[offset:]
	if len(old) > 0 && !bytes.Equal(old, buf.Bytes()) {
		out := strings.TrimSuffix(args[0], ".in") + ".out"
		if err := ioutil.WriteFile(out, append(data[:offset:offset], buf.Bytes()...), 0666); err != nil {
			log.Fatal(err)
		}
		log.Fatalf("goliblink produced different output:\n\toriginal: %s\n\tgoliblink: %s", args[1], out)
	}

	if len(old) == 0 {
		data = append(data, buf.Bytes()...)
		if err := ioutil.WriteFile(args[1], data, 0666); err != nil {
			log.Fatal(err)
		}
	}
}

func rdstring(b *bufio.Reader) string {
	s, err := b.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	s = strings.TrimSpace(s)
	if s == "<nil>" {
		s = ""
	}
	return s
}

func rdhex(b *bufio.Reader) int64 {
	line, err := b.ReadSlice('\n')
	if err != nil {
		log.Fatal(err)
	}
	var v int64
	for i := 0; i < len(line)-1; i++ {
		switch c := line[i]; {
		case '0' <= c && c <= '9':
			v = v*16 + int64(c) - '0'
		case 'a' <= c && c <= 'f':
			v = v*16 + int64(c) - 'a' + 10
		default:
			log.Fatalf("unexpected id %q", line)
		}
	}
	return v
}

func rdint(b *bufio.Reader) int64 {
	line, err := b.ReadSlice('\n')
	if err != nil {
		log.Fatal(err)
	}
	var v int64
	neg := false
	for i := 0; i < len(line)-1; i++ {
		switch c := line[i]; {
		case i == 0 && c == '-':
			neg = true
		case '0' <= c && c <= '9':
			v = v*10 + int64(c) - '0'
		default:
			log.Fatalf("unexpected int %q", line)
		}
	}
	if neg {
		v = -v
	}
	return v
}

func rdplist(b *bufio.Reader) *liblink.Plist {
	id := rdhex(b)
	if id == 0 {
		return nil
	}
	pl := plists[id]
	if pl == nil {
		pl = new(liblink.Plist)
		plists[id] = pl
		undef[pl] = true
	}
	return pl
}

func rdsym(b *bufio.Reader) *liblink.LSym {
	id := rdhex(b)
	if id == 0 {
		return nil
	}
	sym := syms[id]
	if sym == nil {
		sym = new(liblink.LSym)
		syms[id] = sym
		undef[sym] = true
	}
	return sym
}

func rdprog(b *bufio.Reader) *liblink.Prog {
	id := rdhex(b)
	if id == 0 {
		return nil
	}
	prog := progs[id]
	if prog == nil {
		prog = new(liblink.Prog)
		prog.Ctxt = ctxt
		progs[id] = prog
		undef[prog] = true
	}
	return prog
}

func rdhist(b *bufio.Reader) *liblink.Hist {
	id := rdhex(b)
	if id == 0 {
		return nil
	}
	h := hists[id]
	if h == nil {
		h = new(liblink.Hist)
		hists[id] = h
		undef[h] = true
	}
	return h
}

func readplist(b *bufio.Reader, pl *liblink.Plist) {
	if !undef[pl] {
		panic("double-def")
	}
	delete(undef, pl)
	pl.Recur = int(rdint(b))
	pl.Name = rdsym(b)
	pl.Firstpc = rdprog(b)
	pl.Link = rdplist(b)
}

func readsym(b *bufio.Reader, s *liblink.LSym) {
	if !undef[s] {
		panic("double-def")
	}
	delete(undef, s)
	s.Name = rdstring(b)
	s.Extname = rdstring(b)
	s.Type_ = int16(rdint(b))
	s.Version = int16(rdint(b))
	s.Dupok = uint8(rdint(b))
	s.External = uint8(rdint(b))
	s.Nosplit = uint8(rdint(b))
	s.Reachable = uint8(rdint(b))
	s.Cgoexport = uint8(rdint(b))
	s.Special = uint8(rdint(b))
	s.Stkcheck = uint8(rdint(b))
	s.Hide = uint8(rdint(b))
	s.Leaf = uint8(rdint(b))
	s.Fnptr = uint8(rdint(b))
	s.Seenglobl = uint8(rdint(b))
	s.Onlist = uint8(rdint(b))
	s.Symid = int16(rdint(b))
	s.Dynid = int32(rdint(b))
	s.Sig = int32(rdint(b))
	s.Plt = int32(rdint(b))
	s.Got = int32(rdint(b))
	s.Align = int32(rdint(b))
	s.Elfsym = int32(rdint(b))
	s.Args = int32(rdint(b))
	s.Locals = int32(rdint(b))
	s.Value = rdint(b)
	s.Size = rdint(b)
	hashed[rdsym(b)] = true
	s.Allsym = rdsym(b)
	s.Next = rdsym(b)
	s.Sub = rdsym(b)
	s.Outer = rdsym(b)
	s.Gotype = rdsym(b)
	s.Reachparent = rdsym(b)
	s.Queue = rdsym(b)
	s.File = rdstring(b)
	s.Dynimplib = rdstring(b)
	s.Dynimpvers = rdstring(b)
	s.Text = rdprog(b)
	s.Etext = rdprog(b)
	n := int(rdint(b))
	if n > 0 {
		s.P = make([]byte, n)
		io.ReadFull(b, s.P)
	}
	s.R = make([]liblink.Reloc, int(rdint(b)))
	for i := range s.R {
		r := &s.R[i]
		r.Off = int32(rdint(b))
		r.Siz = uint8(rdint(b))
		r.Done = uint8(rdint(b))
		r.Type_ = int32(rdint(b))
		r.Add = rdint(b)
		r.Xadd = rdint(b)
		r.Sym = rdsym(b)
		r.Xsym = rdsym(b)
	}
}

func readprog(b *bufio.Reader, p *liblink.Prog) {
	if !undef[p] {
		panic("double-def")
	}
	delete(undef, p)
	p.Pc = rdint(b)
	p.Lineno = int32(rdint(b))
	p.Link = rdprog(b)
	p.As = int16(rdint(b))
	p.Reg = uint8(rdint(b))
	p.Scond = uint8(rdint(b))
	p.Width = int8(rdint(b))
	readaddr(b, &p.From)
	readaddr(b, &p.To)
}

func readaddr(b *bufio.Reader, a *liblink.Addr) {
	if rdstring(b) != "addr" {
		log.Fatal("out of sync")
	}
	a.Offset = rdint(b)
	a.U.Dval = rdfloat(b)
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = byte(rdint(b))
	}
	a.U.Sval = string(buf)
	a.U.Branch = rdprog(b)
	a.Sym = rdsym(b)
	a.Gotype = rdsym(b)
	a.Type_ = int16(rdint(b))
	a.Index = uint8(rdint(b))
	a.Scale = int8(rdint(b))
	a.Reg = int8(rdint(b))
	a.Name = int8(rdint(b))
	a.Class = int8(rdint(b))
	a.Etype = uint8(rdint(b))
	a.Offset2 = int32(rdint(b))
	a.Width = rdint(b)
}

func readhist(b *bufio.Reader, h *liblink.Hist) {
	if !undef[h] {
		panic("double-def")
	}
	delete(undef, h)
	h.Link = rdhist(b)
	h.Name = rdstring(b)
	h.Line = int32(rdint(b))
	h.Offset = int32(rdint(b))
}

func rdfloat(b *bufio.Reader) float64 {
	return math.Float64frombits(uint64(rdint(b)))
}
