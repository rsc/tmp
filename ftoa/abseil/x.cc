#include "absl_strings_charconv.h"

extern "C" {

double
abslstrtod(const char *s, int n)
{
	double d;
	absl::from_chars(s, s+n, d);
	return d;
}

} // extern "C"