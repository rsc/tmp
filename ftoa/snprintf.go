// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ftoa

/*
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

void
loopgcvt(char *dst, long long n, double f, int prec)
{
	char buf[100];

	for (long long i = 0; i < n; i++)
		gcvt(f, prec, buf);
	strcpy(dst, buf);
}

void
loopsnprintf(char *dst, long long n, double f, int prec)
{
	char buf[100];

	for (long long i = 0; i < n; i++)
		snprintf(buf, sizeof buf, "%.*e", prec-1, f);
	strcpy(dst, buf);
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

func snprintfLoop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopsnprintf((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}
