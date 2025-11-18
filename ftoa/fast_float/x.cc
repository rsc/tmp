#include "fast_float.h"

extern "C" {

double fast_float_strtod(const char *s, int len) {
	double d;
	fast_float::from_chars(s, s+len, d);
	return d;
}

} // extern "C"
