package liblink

// (The comments in this file were copied from the manpage files rune.3,
// isalpharune.3, and runestrcat.3. Some formatting changes were also made
// to conform to Google style. /JRM 11/11/05)

type Fmt struct {
	runes     uint8
	start     interface{}
	to        interface{}
	stop      interface{}
	flush     func(*Fmt) int
	farg      interface{}
	nfmt      int
	args      []interface{}
	r         uint
	width     int
	prec      int
	flags     uint32
	decimal   string
	thousands string
	grouping  string
}

const (
	FmtWidth    = 1
	FmtLeft     = FmtWidth << 1
	FmtPrec     = FmtLeft << 1
	FmtSharp    = FmtPrec << 1
	FmtSpace    = FmtSharp << 1
	FmtSign     = FmtSpace << 1
	FmtApost    = FmtSign << 1
	FmtZero     = FmtApost << 1
	FmtUnsigned = FmtZero << 1
	FmtShort    = FmtUnsigned << 1
	FmtLong     = FmtShort << 1
	FmtVLong    = FmtLong << 1
	FmtComma    = FmtVLong << 1
	FmtByte     = FmtComma << 1
	FmtLDouble  = FmtByte << 1
	FmtFlag     = FmtLDouble << 1
)

var fmtdoquote func(int) int

/* Edit .+1,/^$/ | cfn $PLAN9/src/lib9/fmt/?*.c | grep -v static |grep -v __ */
