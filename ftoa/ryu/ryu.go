package ryu

/*
#cgo CFLAGS: -I..

#include "ryu/ryu.h"

int loopRyuEfmt(long long n, double f, int prec) {
	char buf[100];
	int total;

	total = 0;
	for (long long i = 0; i < n; i++) {
		d2exp_buffered(f, prec, buf);
		total += buf[2];
	}
	return total;
}
*/
import "C"

func LoopEfmt(n int, f float64, prec int) {
	C.loopRyuEfmt(C.longlong(n), C.double(f), C.int(prec))
}
