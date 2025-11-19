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

long long
loopsumdblconv(long long n, double *f, int nf, int prec)
{
	char buf[100];
	double_conversion::StringBuilder b(buf, sizeof buf);
	long long out;

	for (long long i = 0; i < n; i++) {
		long long total = 0;
		for (int j = 0; j < nf; j++) {
			b.Reset();
			if(!double_conversion::DoubleToStringConverter::EcmaScriptConverter().ToExponential(f[j], prec-1, &b))
				abort();
			if(b.position() >= sizeof buf) {
				printf("OOPS %d %.*s\n", b.size(), (int)(sizeof buf), buf);
				abort();
			}
			int n = b.position();
			total += buf[0];
		}
		out = total;
	}
	return out;
}

} // extern "C"

