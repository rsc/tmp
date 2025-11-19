package abseil

/*
#cgo CXXFLAGS: -std=c++17

#include <stdlib.h>
#include <string.h>

double abslstrtod(const char*, int);
double
loopabslstrtod(long long n, char *s)
{
	double f;
	int len = strlen(s);
	for(long long i = 0; i < n; i++)
		f = abslstrtod(s, len);
	return f;
}

double
sumabslstrtod(long long n, char *s)
{
	double f;
	for (long long i = 0; i < n; i++) {
		char *start = s;
		double total = 0.0;
		for (char *p = s; *p; p++) {
			if(*p == '\n') {
				total += abslstrtod(start, p-start);
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

func LoopStrtod(n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.loopabslstrtod(C.longlong(n), p))
}

func LoopSumStrtod(n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.sumabslstrtod(C.longlong(n), p))
}
