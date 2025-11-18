package fast_float

/*
#include <string.h>
#include <stdlib.h>

double fast_float_strtod(const char*, int);

double
loopstrtod(long long n, char *s)
{
	double f;
	int len = strlen(s);
	for(long long i = 0; i < n; i++)
		f = fast_float_strtod(s, len);
	return f;
}

*/
import "C"
import "unsafe"

func LoopStrtod(n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.loopstrtod(C.longlong(n), p))
}
