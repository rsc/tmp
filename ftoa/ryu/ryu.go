package ryu

/*
#cgo CFLAGS: -I..

#include <string.h>
#include "ryu/ryu.h"

void
loopryu(char *dst, long long n, double f, int prec) {
	char buf[100];

	for (long long i = 0; i < n; i++)
		d2exp_buffered(f, prec, buf);
	strcpy(dst, buf);
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
