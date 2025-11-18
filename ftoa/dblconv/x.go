package dblconv

/*
int loopEfmt(long long, double, int);
*/
import "C"

func LoopEfmt(n int, f float64, prec int) {
	C.loopEfmt(C.longlong(n), C.double(f), C.int(prec))
}
