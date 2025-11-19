// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ftoa

/*
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

void loopcxxftoa(char*, long long, double, int);
long long loopsumcxxftoa(long long n, double *f, int nf, int prec);

void
loopgcvt(char *dst, long long n, double f, int prec)
{
	char buf[100];

	for (long long i = 0; i < n; i++)
		gcvt(f, prec, buf);
	strcpy(dst, buf);
}

long long
loopsumgcvt(long long n, double *f, int nf, int prec)
{
	char buf[100];
	long long out;

	for (long long i = 0; i < n; i++) {
		long long total = 0;
		for (int j = 0; j < nf; j++) {
			gcvt(f[j], prec, buf);
			total += buf[0];
		}
		out = total;
	}
	return out;
}

void
loopsnprintf(char *dst, long long n, double f, int prec)
{
	char buf[100];

	for (long long i = 0; i < n; i++)
		snprintf(buf, sizeof buf, "%.*e", prec-1, f);
	strcpy(dst, buf);
}


double
loopstrtod(long long n, char *s)
{
	double f;
	int len = strlen(s);
	for(long long i = 0; i < n; i++)
		f = strtod(s, NULL);
	return f;
}

double
sumstrtod(long long n, char *s)
{
	double f;
	for (long long i = 0; i < n; i++) {
		char *start = s;
		double total = 0.0;
		for (char *p = s; *p; p++) {
			if(*p == '\n') {
				total += strtod(start, NULL);
				start = p+1;
			}
		}
		f = total;
	}
	return f;
}

*/
import "C"
import "unsafe"

func gcvtLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopgcvt((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func cxxLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopcxxftoa((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func gcvtLoopSum(n int, fs []float64, prec int) int64 {
	return int64(C.loopsumgcvt(C.longlong(n), (*C.double)(&fs[0]), C.int(len(fs)), C.int(prec)))
}

func cxxLoopSum(n int, fs []float64, prec int) int64 {
	return int64(C.loopsumcxxftoa(C.longlong(n), (*C.double)(&fs[0]), C.int(len(fs)), C.int(prec)))
}

func snprintfLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopsnprintf((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func strtodLoop(n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.loopstrtod(C.longlong(n), p))
}

func strtodLoopSum(n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.sumstrtod(C.longlong(n), p))
}
