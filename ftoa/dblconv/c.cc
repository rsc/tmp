#include "double-to-string.h"
#include <stdlib.h>

extern "C" {

int
loopEfmt(long long n, double f, int prec)
{
	char buf[100];
	int total;
	double_conversion::StringBuilder b(buf, sizeof buf);

	total = 0;
	for (long long i = 0; i < n; i++) {
		b.Reset();
		if(!double_conversion::DoubleToStringConverter::EcmaScriptConverter().ToExponential(f, prec, &b))
			abort();
		total += buf[2];
	}
	return total;
}

} // extern "C"

