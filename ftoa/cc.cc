#include <iostream>
#include <string>
#include <sstream>
#include <iomanip>
#include <string.h>

extern "C" {

void loopcxxftoa(char *dst, long long n, double f, int prec) {
	for (long long i = 0; i < n; i++) {
		std::ostringstream ss;
		ss << std::scientific << std::setprecision(prec-1) << f;
		std::string s = ss.str();
		memmove(dst, s.data(), s.size());
		dst[s.size()] = 0;
	}
}

long long loopsumcxxftoa(long long n, double *f, int nf, int prec) {
	long long out = 0;
	for (long long i = 0; i < n; i++) {
		long long total = 0;
		for (int j = 0; j < nf; j++) {
			std::ostringstream ss;
			ss << std::scientific << std::setprecision(prec-1) << f[j];
			std::string s = ss.str();
			total += s[0];
		}
		out = total;
	}
	return out;
}

} // extern "C"
