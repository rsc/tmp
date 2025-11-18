#include "double-to-string.h"
#include <stdlib.h>
#include <stdio.h>

extern "C" {

void
loopdblconv(char *dst, long long n, double f, int prec)
{
	char buf[100];
	double_conversion::StringBuilder b(buf, sizeof buf);

	for (long long i = 0; i < n; i++) {
		b.Reset();
		if(!double_conversion::DoubleToStringConverter::EcmaScriptConverter().ToExponential(f, prec-1, &b))
			abort();
		if(b.position() >= sizeof buf) {
			printf("OOPS %d %.*s\n", b.size(), (int)(sizeof buf), buf);
			abort();
		}
		buf[b.position()] = 0;
	}
	strcpy(dst, buf);
}

} // extern "C"

