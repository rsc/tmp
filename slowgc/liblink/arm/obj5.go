package arm

import (
	"rsc.io/tmp/slowgc/liblink"
	"encoding/binary"
	"fmt"
	"log"
	"math"
)

// Derived from Inferno utils/5c/swt.c
// http://code.google.com/p/inferno-os/source/browse/utils/5c/swt.c
//
//	Copyright © 1994-1999 Lucent Technologies Inc.  All rights reserved.
//	Portions Copyright © 1995-1997 C H Forsyth (forsyth@terzarima.net)
//	Portions Copyright © 1997-1999 Vita Nuova Limited
//	Portions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com)
//	Portions Copyright © 2004,2006 Bruce Ellis
//	Portions Copyright © 2005-2007 C H Forsyth (forsyth@terzarima.net)
//	Revisions Copyright © 2000-2007 Lucent Technologies Inc. and others
//	Portions Copyright © 2009 The Go Authors.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

var zprg5 = liblink.Prog{
	As:    AGOK,
	Scond: C_SCOND_NONE,
	Reg:   NREG,
	From: liblink.Addr{
		Name:  D_NONE,
		Type_: D_NONE,
		Reg:   NREG,
	},
	To: liblink.Addr{
		Name:  D_NONE,
		Type_: D_NONE,
		Reg:   NREG,
	},
}

func symtype(a *liblink.Addr) int {
	return int(a.Name)
}

func isdata(p *liblink.Prog) bool {
	return p.As == ADATA || p.As == AGLOBL
}

func iscall(p *liblink.Prog) bool {
	return p.As == ABL
}

func datasize(p *liblink.Prog) int {
	return int(p.Reg)
}

func textflag(p *liblink.Prog) int {
	return int(p.Reg)
}

func settextflag(p *liblink.Prog, f int) {
	p.Reg = uint8(f)
}

func progedit(ctxt *liblink.Link, p *liblink.Prog) {
	var literal string
	var s *liblink.LSym
	var tlsfallback *liblink.LSym

	p.From.Class = 0
	p.To.Class = 0

	// Rewrite B/BL to symbol as D_BRANCH.
	switch p.As {

	case AB,
		ABL,
		ADUFFZERO,
		ADUFFCOPY:
		if p.To.Type_ == D_OREG && (p.To.Name == D_EXTERN || p.To.Name == D_STATIC) && p.To.Sym != nil {
			p.To.Type_ = D_BRANCH
		}
		break
	}

	// Replace TLS register fetches on older ARM procesors.
	switch p.As {

	// Treat MRC 15, 0, <reg>, C13, C0, 3 specially.
	case AMRC:
		if p.To.Offset&0xffff0fff == 0xee1d0f70 {

			// Because the instruction might be rewriten to a BL which returns in R0
			// the register must be zero.
			if p.To.Offset&0xf000 != 0 {

				ctxt.Diag("%L: TLS MRC instruction must write to R0 as it might get translated into a BL instruction", p.Lineno)
			}

			if ctxt.Goarm < 7 {
				// Replace it with BL runtime.read_tls_fallback(SB) for ARM CPUs that lack the tls extension.
				if tlsfallback == nil {

					tlsfallback = liblink.Linklookup(ctxt, "runtime.read_tls_fallback", 0)
				}

				// MOVW	LR, R11
				p.As = AMOVW

				p.From.Type_ = D_REG
				p.From.Reg = REGLINK
				p.To.Type_ = D_REG
				p.To.Reg = REGTMP

				// BL	runtime.read_tls_fallback(SB)
				p = liblink.Appendp(ctxt, p)

				p.As = ABL
				p.To.Type_ = D_BRANCH
				p.To.Sym = tlsfallback
				p.To.Offset = 0

				// MOVW	R11, LR
				p = liblink.Appendp(ctxt, p)

				p.As = AMOVW
				p.From.Type_ = D_REG
				p.From.Reg = REGTMP
				p.To.Type_ = D_REG
				p.To.Reg = REGLINK
				break
			}
		}

		// Otherwise, MRC/MCR instructions need no further treatment.
		p.As = AWORD

		break
	}

	// Rewrite float constants to values stored in memory.
	switch p.As {

	case AMOVF:
		if p.From.Type_ == D_FCONST && chipfloat5(ctxt, p.From.U.Dval) < 0 && (chipzero5(ctxt, p.From.U.Dval) < 0 || p.Scond&C_SCOND != C_SCOND_NONE) {
			var i32 uint32
			var f32 float32
			f32 = float32(p.From.U.Dval)
			i32 = math.Float32bits(f32)
			literal = fmt.Sprintf("$f32.%08x", i32)
			s = liblink.Linklookup(ctxt, literal, 0)
			if s.Type_ == 0 {
				s.Type_ = liblink.SRODATA
				liblink.Adduint32(ctxt, s, i32)
				s.Reachable = 0
			}

			p.From.Type_ = D_OREG
			p.From.Sym = s
			p.From.Name = D_EXTERN
			p.From.Offset = 0
		}

	case AMOVD:
		if p.From.Type_ == D_FCONST && chipfloat5(ctxt, p.From.U.Dval) < 0 && (chipzero5(ctxt, p.From.U.Dval) < 0 || p.Scond&C_SCOND != C_SCOND_NONE) {
			var i64 uint64
			i64 = math.Float64bits(p.From.U.Dval)
			literal = fmt.Sprintf("$f64.%016x", i64)
			s = liblink.Linklookup(ctxt, literal, 0)
			if s.Type_ == 0 {
				s.Type_ = liblink.SRODATA
				liblink.Adduint64(ctxt, s, i64)
				s.Reachable = 0
			}

			p.From.Type_ = D_OREG
			p.From.Sym = s
			p.From.Name = D_EXTERN
			p.From.Offset = 0
		}

		break
	}

	if ctxt.Flag_shared != 0 {
		// Shared libraries use R_ARM_TLS_IE32 instead of
		// R_ARM_TLS_LE32, replacing the link time constant TLS offset in
		// runtime.tlsg with an address to a GOT entry containing the
		// offset. Rewrite $runtime.tlsg(SB) to runtime.tlsg(SB) to
		// compensate.
		if ctxt.Tlsg == nil {

			ctxt.Tlsg = liblink.Linklookup(ctxt, "runtime.tlsg", 0)
		}

		if p.From.Type_ == D_CONST && p.From.Name == D_EXTERN && p.From.Sym == ctxt.Tlsg {
			p.From.Type_ = D_OREG
		}
		if p.To.Type_ == D_CONST && p.To.Name == D_EXTERN && p.To.Sym == ctxt.Tlsg {
			p.To.Type_ = D_OREG
		}
	}
}

func prg() *liblink.Prog {
	var p *liblink.Prog

	p = new(liblink.Prog)
	*p = zprg5
	return p
}

// Prog.mark
const (
	FOLL  = 1 << 0
	LABEL = 1 << 1
	LEAF  = 1 << 2
)

func linkcase(casep *liblink.Prog) {
	var p *liblink.Prog

	for p = casep; p != nil; p = p.Link {
		if p.As == ABCASE {
			for ; p != nil && p.As == ABCASE; p = p.Link {
				p.Pcrel = casep
			}
			break
		}
	}
}

func nocache5(p *liblink.Prog) {
	p.Optab = 0
	p.From.Class = 0
	p.To.Class = 0
}

func addstacksplit(ctxt *liblink.Link, cursym *liblink.LSym) {
	var p *liblink.Prog
	var pl *liblink.Prog
	var p1 *liblink.Prog
	var p2 *liblink.Prog
	var q *liblink.Prog
	var q1 *liblink.Prog
	var q2 *liblink.Prog
	var o int
	var autosize int32
	var autoffset int32

	autosize = 0

	if ctxt.Symmorestack[0] == nil {
		ctxt.Symmorestack[0] = liblink.Linklookup(ctxt, "runtime.morestack", 0)
		ctxt.Symmorestack[1] = liblink.Linklookup(ctxt, "runtime.morestack_noctxt", 0)
	}

	q = nil

	ctxt.Cursym = cursym

	if cursym.Text == nil || cursym.Text.Link == nil {
		return
	}

	softfloat(ctxt, cursym)

	p = cursym.Text
	autoffset = int32(p.To.Offset)
	if autoffset < 0 {
		autoffset = 0
	}
	cursym.Locals = autoffset
	cursym.Args = p.To.Offset2

	if ctxt.Debugzerostack != 0 {
		if autoffset != 0 && !(p.Reg&liblink.NOSPLIT != 0) {
			// MOVW $4(R13), R1
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.From.Type_ = D_CONST
			p.From.Reg = 13
			p.From.Offset = 4
			p.To.Type_ = D_REG
			p.To.Reg = 1

			// MOVW $n(R13), R2
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.From.Type_ = D_CONST
			p.From.Reg = 13
			p.From.Offset = 4 + int64(autoffset)
			p.To.Type_ = D_REG
			p.To.Reg = 2

			// MOVW $0, R3
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.From.Type_ = D_CONST
			p.From.Offset = 0
			p.To.Type_ = D_REG
			p.To.Reg = 3

			// L:
			//	MOVW.nil R3, 0(R1) +4
			//	CMP R1, R2
			//	BNE L
			pl = liblink.Appendp(ctxt, p)
			p = pl

			p.As = AMOVW
			p.From.Type_ = D_REG
			p.From.Reg = 3
			p.To.Type_ = D_OREG
			p.To.Reg = 1
			p.To.Offset = 4
			p.Scond |= C_PBIT

			p = liblink.Appendp(ctxt, p)
			p.As = ACMP
			p.From.Type_ = D_REG
			p.From.Reg = 1
			p.Reg = 2

			p = liblink.Appendp(ctxt, p)
			p.As = ABNE
			p.To.Type_ = D_BRANCH
			p.Pcond = pl
		}
	}

	/*
	 * find leaf subroutines
	 * strip NOPs
	 * expand RET
	 * expand BECOME pseudo
	 */
	for p = cursym.Text; p != nil; p = p.Link {

		switch p.As {
		case ACASE:
			if ctxt.Flag_shared != 0 {
				linkcase(p)
			}

		case ATEXT:
			p.Mark |= LEAF

		case ARET:
			break

		case ADIV,
			ADIVU,
			AMOD,
			AMODU:
			q = p
			if ctxt.Sym_div == nil {
				initdiv(ctxt)
			}
			cursym.Text.Mark &^= LEAF
			continue

		case ANOP:
			q1 = p.Link
			q.Link = q1 /* q is non-nop */
			if q1 != nil {
				q1.Mark |= p.Mark
			}
			continue

		case ABL,
			ABX,
			ADUFFZERO,
			ADUFFCOPY:
			cursym.Text.Mark &^= LEAF
			fallthrough

		case ABCASE,
			AB,
			ABEQ,
			ABNE,
			ABCS,
			ABHS,
			ABCC,
			ABLO,
			ABMI,
			ABPL,
			ABVS,
			ABVC,
			ABHI,
			ABLS,
			ABGE,
			ABLT,
			ABGT,
			ABLE:
			q1 = p.Pcond
			if q1 != nil {
				for q1.As == ANOP {
					q1 = q1.Link
					p.Pcond = q1
				}
			}

			break
		}

		q = p
	}

	for p = cursym.Text; p != nil; p = p.Link {
		o = int(p.As)
		switch o {
		case ATEXT:
			autosize = int32(p.To.Offset + 4)
			if autosize <= 4 {
				if cursym.Text.Mark&LEAF != 0 {
					p.To.Offset = -4
					autosize = 0
				}
			}

			if !(autosize != 0) && !(cursym.Text.Mark&LEAF != 0) {
				if ctxt.Debugvlog != 0 {
					fmt.Fprintf(ctxt.Bso, "save suppressed in: %s\n", cursym.Name)
					liblink.Bflush(ctxt.Bso)
				}

				cursym.Text.Mark |= LEAF
			}

			if cursym.Text.Mark&LEAF != 0 {
				cursym.Leaf = 1
				if !(autosize != 0) {
					break
				}
			}

			if !(p.Reg&liblink.NOSPLIT != 0) {
				p = stacksplit(ctxt, p, autosize, bool2int(!(cursym.Text.Reg&liblink.NEEDCTXT != 0))) // emit split check
			}

			// MOVW.W		R14,$-autosize(SP)
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.Scond |= C_WBIT
			p.From.Type_ = D_REG
			p.From.Reg = REGLINK
			p.To.Type_ = D_OREG
			p.To.Offset = int64(-autosize)
			p.To.Reg = REGSP
			p.Spadj = autosize

			if cursym.Text.Reg&liblink.WRAPPER != 0 {
				// if(g->panic != nil && g->panic->argp == FP) g->panic->argp = bottom-of-frame
				//
				//	MOVW g_panic(g), R1
				//	CMP $0, R1
				//	B.EQ end
				//	MOVW panic_argp(R1), R2
				//	ADD $(autosize+4), R13, R3
				//	CMP R2, R3
				//	B.NE end
				//	ADD $4, R13, R4
				//	MOVW R4, panic_argp(R1)
				// end:
				//	NOP
				//
				// The NOP is needed to give the jumps somewhere to land.
				// It is a liblink NOP, not an ARM NOP: it encodes to 0 instruction bytes.

				p = liblink.Appendp(ctxt, p)

				p.As = AMOVW
				p.From.Type_ = D_OREG
				p.From.Reg = REGG
				p.From.Offset = 4 * int64(ctxt.Arch.Ptrsize) // G.panic
				p.To.Type_ = D_REG
				p.To.Reg = 1

				p = liblink.Appendp(ctxt, p)
				p.As = ACMP
				p.From.Type_ = D_CONST
				p.From.Offset = 0
				p.Reg = 1

				p = liblink.Appendp(ctxt, p)
				p.As = ABEQ
				p.To.Type_ = D_BRANCH
				p1 = p

				p = liblink.Appendp(ctxt, p)
				p.As = AMOVW
				p.From.Type_ = D_OREG
				p.From.Reg = 1
				p.From.Offset = 0 // Panic.argp
				p.To.Type_ = D_REG
				p.To.Reg = 2

				p = liblink.Appendp(ctxt, p)
				p.As = AADD
				p.From.Type_ = D_CONST
				p.From.Offset = int64(autosize) + 4
				p.Reg = 13
				p.To.Type_ = D_REG
				p.To.Reg = 3

				p = liblink.Appendp(ctxt, p)
				p.As = ACMP
				p.From.Type_ = D_REG
				p.From.Reg = 2
				p.Reg = 3

				p = liblink.Appendp(ctxt, p)
				p.As = ABNE
				p.To.Type_ = D_BRANCH
				p2 = p

				p = liblink.Appendp(ctxt, p)
				p.As = AADD
				p.From.Type_ = D_CONST
				p.From.Offset = 4
				p.Reg = 13
				p.To.Type_ = D_REG
				p.To.Reg = 4

				p = liblink.Appendp(ctxt, p)
				p.As = AMOVW
				p.From.Type_ = D_REG
				p.From.Reg = 4
				p.To.Type_ = D_OREG
				p.To.Reg = 1
				p.To.Offset = 0 // Panic.argp

				p = liblink.Appendp(ctxt, p)

				p.As = ANOP
				p1.Pcond = p
				p2.Pcond = p
			}

		case ARET:
			nocache5(p)
			if cursym.Text.Mark&LEAF != 0 {
				if !(autosize != 0) {
					p.As = AB
					p.From = zprg5.From
					if p.To.Sym != nil { // retjmp
						p.To.Type_ = D_BRANCH
					} else {

						p.To.Type_ = D_OREG
						p.To.Offset = 0
						p.To.Reg = REGLINK
					}

					break
				}
			}

			p.As = AMOVW
			p.Scond |= C_PBIT
			p.From.Type_ = D_OREG
			p.From.Offset = int64(autosize)
			p.From.Reg = REGSP
			p.To.Type_ = D_REG
			p.To.Reg = REGPC

			// If there are instructions following
			// this ARET, they come from a branch
			// with the same stackframe, so no spadj.
			if p.To.Sym != nil { // retjmp
				p.To.Reg = REGLINK
				q2 = liblink.Appendp(ctxt, p)
				q2.As = AB
				q2.To.Type_ = D_BRANCH
				q2.To.Sym = p.To.Sym
				p.To.Sym = nil
				p = q2
			}

		case AADD:
			if p.From.Type_ == D_CONST && p.From.Reg == NREG && p.To.Type_ == D_REG && p.To.Reg == REGSP {
				p.Spadj = int32(-p.From.Offset)
			}

		case ASUB:
			if p.From.Type_ == D_CONST && p.From.Reg == NREG && p.To.Type_ == D_REG && p.To.Reg == REGSP {
				p.Spadj = int32(p.From.Offset)
			}

		case ADIV,
			ADIVU,
			AMOD,
			AMODU:
			if ctxt.Debugdivmod != 0 {
				break
			}
			if p.From.Type_ != D_REG {
				break
			}
			if p.To.Type_ != D_REG {
				break
			}
			q1 = p

			/* MOV a,4(SP) */
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.Lineno = q1.Lineno
			p.From.Type_ = D_REG
			p.From.Reg = q1.From.Reg
			p.To.Type_ = D_OREG
			p.To.Reg = REGSP
			p.To.Offset = 4

			/* MOV b,REGTMP */
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.Lineno = q1.Lineno
			p.From.Type_ = D_REG
			p.From.Reg = int8(q1.Reg)
			if q1.Reg == NREG {
				p.From.Reg = q1.To.Reg
			}
			p.To.Type_ = D_REG
			p.To.Reg = REGTMP
			p.To.Offset = 0

			/* CALL appropriate */
			p = liblink.Appendp(ctxt, p)

			p.As = ABL
			p.Lineno = q1.Lineno
			p.To.Type_ = D_BRANCH
			switch o {
			case ADIV:
				p.To.Sym = ctxt.Sym_div

			case ADIVU:
				p.To.Sym = ctxt.Sym_divu

			case AMOD:
				p.To.Sym = ctxt.Sym_mod

			case AMODU:
				p.To.Sym = ctxt.Sym_modu
				break
			}

			/* MOV REGTMP, b */
			p = liblink.Appendp(ctxt, p)

			p.As = AMOVW
			p.Lineno = q1.Lineno
			p.From.Type_ = D_REG
			p.From.Reg = REGTMP
			p.From.Offset = 0
			p.To.Type_ = D_REG
			p.To.Reg = q1.To.Reg

			/* ADD $8,SP */
			p = liblink.Appendp(ctxt, p)

			p.As = AADD
			p.Lineno = q1.Lineno
			p.From.Type_ = D_CONST
			p.From.Reg = NREG
			p.From.Offset = 8
			p.Reg = NREG
			p.To.Type_ = D_REG
			p.To.Reg = REGSP
			p.Spadj = -8

			/* Keep saved LR at 0(SP) after SP change. */
			/* MOVW 0(SP), REGTMP; MOVW REGTMP, -8!(SP) */
			/* TODO: Remove SP adjustments; see issue 6699. */
			q1.As = AMOVW

			q1.From.Type_ = D_OREG
			q1.From.Reg = REGSP
			q1.From.Offset = 0
			q1.Reg = NREG
			q1.To.Type_ = D_REG
			q1.To.Reg = REGTMP

			/* SUB $8,SP */
			q1 = liblink.Appendp(ctxt, q1)

			q1.As = AMOVW
			q1.From.Type_ = D_REG
			q1.From.Reg = REGTMP
			q1.Reg = NREG
			q1.To.Type_ = D_OREG
			q1.To.Reg = REGSP
			q1.To.Offset = -8
			q1.Scond |= C_WBIT
			q1.Spadj = 8

		case AMOVW:
			if (p.Scond&C_WBIT != 0) && p.To.Type_ == D_OREG && p.To.Reg == REGSP {
				p.Spadj = int32(-p.To.Offset)
			}
			if (p.Scond&C_PBIT != 0) && p.From.Type_ == D_OREG && p.From.Reg == REGSP && p.To.Reg != REGPC {
				p.Spadj = int32(-p.From.Offset)
			}
			if p.From.Type_ == D_CONST && p.From.Reg == REGSP && p.To.Type_ == D_REG && p.To.Reg == REGSP {
				p.Spadj = int32(-p.From.Offset)
			}
			break
		}
	}
}

func softfloat(ctxt *liblink.Link, cursym *liblink.LSym) {
	var p *liblink.Prog
	var next *liblink.Prog
	var symsfloat *liblink.LSym
	var wasfloat int

	if ctxt.Goarm > 5 {
		return
	}

	symsfloat = liblink.Linklookup(ctxt, "_sfloat", 0)

	wasfloat = 0
	for p = cursym.Text; p != nil; p = p.Link {
		if p.Pcond != nil {
			p.Pcond.Mark |= LABEL
		}
	}
	for p = cursym.Text; p != nil; p = p.Link {
		switch p.As {
		case AMOVW:
			if p.To.Type_ == D_FREG || p.From.Type_ == D_FREG {
				goto soft
			}
			goto notsoft

		case AMOVWD,
			AMOVWF,
			AMOVDW,
			AMOVFW,
			AMOVFD,
			AMOVDF,
			AMOVF,
			AMOVD,
			ACMPF,
			ACMPD,
			AADDF,
			AADDD,
			ASUBF,
			ASUBD,
			AMULF,
			AMULD,
			ADIVF,
			ADIVD,
			ASQRTF,
			ASQRTD,
			AABSF,
			AABSD:
			goto soft

		default:
			goto notsoft
		}

	soft:
		if !(wasfloat != 0) || (p.Mark&LABEL != 0) {
			next = ctxt.Arch.Prg()
			*next = *p

			// BL _sfloat(SB)
			*p = zprg5

			p.Link = next
			p.As = ABL
			p.To.Type_ = D_BRANCH
			p.To.Sym = symsfloat
			p.Lineno = next.Lineno

			p = next
			wasfloat = 1
		}

		continue

	notsoft:
		wasfloat = 0
	}
}

func stacksplit(ctxt *liblink.Link, p *liblink.Prog, framesize int32, noctxt int) *liblink.Prog {
	// MOVW			g_stackguard(g), R1
	p = liblink.Appendp(ctxt, p)

	p.As = AMOVW
	p.From.Type_ = D_OREG
	p.From.Reg = REGG
	p.From.Offset = 2 * int64(ctxt.Arch.Ptrsize) // G.stackguard0
	if ctxt.Cursym.Cfunc != 0 {
		p.From.Offset = 3 * int64(ctxt.Arch.Ptrsize) // G.stackguard1
	}
	p.To.Type_ = D_REG
	p.To.Reg = 1

	if framesize <= liblink.StackSmall {
		// small stack: SP < stackguard
		//	CMP	stackguard, SP
		p = liblink.Appendp(ctxt, p)

		p.As = ACMP
		p.From.Type_ = D_REG
		p.From.Reg = 1
		p.Reg = REGSP
	} else if framesize <= liblink.StackBig {
		// large stack: SP-framesize < stackguard-StackSmall
		//	MOVW $-framesize(SP), R2
		//	CMP stackguard, R2
		p = liblink.Appendp(ctxt, p)

		p.As = AMOVW
		p.From.Type_ = D_CONST
		p.From.Reg = REGSP
		p.From.Offset = int64(-framesize)
		p.To.Type_ = D_REG
		p.To.Reg = 2

		p = liblink.Appendp(ctxt, p)
		p.As = ACMP
		p.From.Type_ = D_REG
		p.From.Reg = 1
		p.Reg = 2
	} else {

		// Such a large stack we need to protect against wraparound
		// if SP is close to zero.
		//	SP-stackguard+StackGuard < framesize + (StackGuard-StackSmall)
		// The +StackGuard on both sides is required to keep the left side positive:
		// SP is allowed to be slightly below stackguard. See stack.h.
		//	CMP $StackPreempt, R1
		//	MOVW.NE $StackGuard(SP), R2
		//	SUB.NE R1, R2
		//	MOVW.NE $(framesize+(StackGuard-StackSmall)), R3
		//	CMP.NE R3, R2
		p = liblink.Appendp(ctxt, p)

		p.As = ACMP
		p.From.Type_ = D_CONST
		p.From.Offset = int64(uint32(liblink.StackPreempt & (1<<32 - 1)))
		p.Reg = 1

		p = liblink.Appendp(ctxt, p)
		p.As = AMOVW
		p.From.Type_ = D_CONST
		p.From.Reg = REGSP
		p.From.Offset = liblink.StackGuard
		p.To.Type_ = D_REG
		p.To.Reg = 2
		p.Scond = C_SCOND_NE

		p = liblink.Appendp(ctxt, p)
		p.As = ASUB
		p.From.Type_ = D_REG
		p.From.Reg = 1
		p.To.Type_ = D_REG
		p.To.Reg = 2
		p.Scond = C_SCOND_NE

		p = liblink.Appendp(ctxt, p)
		p.As = AMOVW
		p.From.Type_ = D_CONST
		p.From.Offset = int64(framesize) + (liblink.StackGuard - liblink.StackSmall)
		p.To.Type_ = D_REG
		p.To.Reg = 3
		p.Scond = C_SCOND_NE

		p = liblink.Appendp(ctxt, p)
		p.As = ACMP
		p.From.Type_ = D_REG
		p.From.Reg = 3
		p.Reg = 2
		p.Scond = C_SCOND_NE
	}

	// MOVW.LS	R14, R3
	p = liblink.Appendp(ctxt, p)

	p.As = AMOVW
	p.Scond = C_SCOND_LS
	p.From.Type_ = D_REG
	p.From.Reg = REGLINK
	p.To.Type_ = D_REG
	p.To.Reg = 3

	// BL.LS		runtime.morestack(SB) // modifies LR, returns with LO still asserted
	p = liblink.Appendp(ctxt, p)

	p.As = ABL
	p.Scond = C_SCOND_LS
	p.To.Type_ = D_BRANCH
	if ctxt.Cursym.Cfunc != 0 {
		p.To.Sym = liblink.Linklookup(ctxt, "runtime.morestackc", 0)
	} else {

		p.To.Sym = ctxt.Symmorestack[noctxt]
	}

	// BLS	start
	p = liblink.Appendp(ctxt, p)

	p.As = ABLS
	p.To.Type_ = D_BRANCH
	p.Pcond = ctxt.Cursym.Text.Link

	return p
}

func initdiv(ctxt *liblink.Link) {
	if ctxt.Sym_div != nil {
		return
	}
	ctxt.Sym_div = liblink.Linklookup(ctxt, "_div", 0)
	ctxt.Sym_divu = liblink.Linklookup(ctxt, "_divu", 0)
	ctxt.Sym_mod = liblink.Linklookup(ctxt, "_mod", 0)
	ctxt.Sym_modu = liblink.Linklookup(ctxt, "_modu", 0)
}

func follow(ctxt *liblink.Link, s *liblink.LSym) {
	var firstp *liblink.Prog
	var lastp *liblink.Prog

	ctxt.Cursym = s

	firstp = ctxt.Arch.Prg()
	lastp = firstp
	xfol(ctxt, s.Text, &lastp)
	lastp.Link = nil
	s.Text = firstp.Link
}

func relinv(a int) int {
	switch a {
	case ABEQ:
		return ABNE
	case ABNE:
		return ABEQ
	case ABCS:
		return ABCC
	case ABHS:
		return ABLO
	case ABCC:
		return ABCS
	case ABLO:
		return ABHS
	case ABMI:
		return ABPL
	case ABPL:
		return ABMI
	case ABVS:
		return ABVC
	case ABVC:
		return ABVS
	case ABHI:
		return ABLS
	case ABLS:
		return ABHI
	case ABGE:
		return ABLT
	case ABLT:
		return ABGE
	case ABGT:
		return ABLE
	case ABLE:
		return ABGT
	}

	log.Fatalf("unknown relation: %s", anames5[a])
	return 0
}

func xfol(ctxt *liblink.Link, p *liblink.Prog, last **liblink.Prog) {
	var q *liblink.Prog
	var r *liblink.Prog
	var a int
	var i int

loop:
	if p == nil {
		return
	}
	a = int(p.As)
	if a == AB {
		q = p.Pcond
		if q != nil && q.As != ATEXT {
			p.Mark |= FOLL
			p = q
			if !(p.Mark&FOLL != 0) {
				goto loop
			}
		}
	}

	if p.Mark&FOLL != 0 {
		i = 0
		q = p
		for ; i < 4; (func() { i++; q = q.Link })() {
			if q == *last || q == nil {
				break
			}
			a = int(q.As)
			if a == ANOP {
				i--
				continue
			}

			if a == AB || (a == ARET && q.Scond == C_SCOND_NONE) || a == ARFE || a == AUNDEF {
				goto copy
			}
			if q.Pcond == nil || (q.Pcond.Mark&FOLL != 0) {
				continue
			}
			if a != ABEQ && a != ABNE {
				continue
			}

		copy:
			for {
				r = ctxt.Arch.Prg()
				*r = *p
				if !(r.Mark&FOLL != 0) {
					fmt.Printf("can't happen 1\n")
				}
				r.Mark |= FOLL
				if p != q {
					p = p.Link
					(*last).Link = r
					*last = r
					continue
				}

				(*last).Link = r
				*last = r
				if a == AB || (a == ARET && q.Scond == C_SCOND_NONE) || a == ARFE || a == AUNDEF {
					return
				}
				r.As = ABNE
				if a == ABNE {
					r.As = ABEQ
				}
				r.Pcond = p.Link
				r.Link = p.Pcond
				if !(r.Link.Mark&FOLL != 0) {
					xfol(ctxt, r.Link, last)
				}
				if !(r.Pcond.Mark&FOLL != 0) {
					fmt.Printf("can't happen 2\n")
				}
				return
			}
		}

		a = AB
		q = ctxt.Arch.Prg()
		q.As = int16(a)
		q.Lineno = p.Lineno
		q.To.Type_ = D_BRANCH
		q.To.Offset = p.Pc
		q.Pcond = p
		p = q
	}

	p.Mark |= FOLL
	(*last).Link = p
	*last = p
	if a == AB || (a == ARET && p.Scond == C_SCOND_NONE) || a == ARFE || a == AUNDEF {
		return
	}

	if p.Pcond != nil {
		if a != ABL && a != ABX && p.Link != nil {
			q = liblink.Brchain(ctxt, p.Link)
			if a != ATEXT && a != ABCASE {
				if q != nil && (q.Mark&FOLL != 0) {
					p.As = int16(relinv(a))
					p.Link = p.Pcond
					p.Pcond = q
				}
			}

			xfol(ctxt, p.Link, last)
			q = liblink.Brchain(ctxt, p.Pcond)
			if q == nil {
				q = p.Pcond
			}
			if q.Mark&FOLL != 0 {
				p.Pcond = q
				return
			}

			p = q
			goto loop
		}
	}

	p = p.Link
	goto loop
}

var Linkarm = liblink.LinkArch{
	ByteOrder:     binary.LittleEndian,
	Pconv:         Pconv,
	Name:          "arm",
	Thechar:       '5',
	Endian:        liblink.LittleEndian,
	Addstacksplit: addstacksplit,
	Assemble:      span5,
	Datasize:      datasize,
	Follow:        follow,
	Iscall:        iscall,
	Isdata:        isdata,
	Prg:           prg,
	Progedit:      progedit,
	Settextflag:   settextflag,
	Symtype:       symtype,
	Textflag:      textflag,
	Minlc:         4,
	Ptrsize:       4,
	Regsize:       4,
	D_ADDR:        D_ADDR,
	D_AUTO:        D_AUTO,
	D_BRANCH:      D_BRANCH,
	D_CONST:       D_CONST,
	D_EXTERN:      D_EXTERN,
	D_FCONST:      D_FCONST,
	D_NONE:        D_NONE,
	D_PARAM:       D_PARAM,
	D_SCONST:      D_SCONST,
	D_STATIC:      D_STATIC,
	D_OREG:        D_OREG,
	ACALL:         ABL,
	ADATA:         ADATA,
	AEND:          AEND,
	AFUNCDATA:     AFUNCDATA,
	AGLOBL:        AGLOBL,
	AJMP:          AB,
	ANOP:          ANOP,
	APCDATA:       APCDATA,
	ARET:          ARET,
	ATEXT:         ATEXT,
	ATYPE:         ATYPE,
	AUSEFIELD:     AUSEFIELD,
}
