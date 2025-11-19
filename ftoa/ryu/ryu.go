package ryu

/*
#cgo CFLAGS: -I..

#include <string.h>
#include "ryu/ryu.h"

void
loopryu(char *dst, long long n, double f, int prec)
{
	char buf[100];

	for (long long i = 0; i < n; i++)
		d2exp_buffered(f, prec, buf);
	strcpy(dst, buf);
}

long long
loopsumryu(long long n, double *f, int nf, int prec)
{
	char buf[100];
	long long out;

	for (long long i = 0; i < n; i++) {
		long long total = 0;
		for (int j = 0; j < nf; j++) {
			int n = d2exp_buffered_n(f[j], prec, buf);
			total += buf[0];
		}
		out = total;
	}
	return out;
}

*/
import "C"
import "unsafe"

func Loop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopryu((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec-1))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func LoopSum(n int, fs []float64, prec int) int64 {
	return int64(C.loopsumryu(C.longlong(n), (*C.double)(&fs[0]), C.int(len(fs)), C.int(prec-1)))
}
