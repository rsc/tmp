// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ftoa

/*
#include <stdio.h>

int
loopSnprintf(long long n, double f, int prec)
{
	int total;
	char buf[100];

	total = 0;
	for (long long i = 0; i < n; i++) {
		snprintf(buf, sizeof buf, "%.*e", prec, f);
		total += buf[2];
	}
	return total;
}

int
loopSnprintd(long long n, long long d)
{
	int total;
	char buf[100];

	total = 0;
	for (long long i = 0; i < n; i++) {
		snprintf(buf, sizeof buf, "%lld", d);
		total += buf[2];
	}
	return total;
}
*/
import "C"

func init() {
	loopSnprintf = _loopSnprintf
	loopSnprintd = _loopSnprintd
}

func _loopSnprintf(n int, f float64, prec int) {
	C.loopSnprintf(C.longlong(n), C.double(f), C.int(prec))
}

func _loopSnprintd(n int, d int64) {
	C.loopSnprintd(C.longlong(n), C.longlong(d))
}
