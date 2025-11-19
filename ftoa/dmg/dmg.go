package dmg

/*
#cgo CFLAGS: -DIEEE_8087 -DLong=int -w

#include <string.h>
#include <stdlib.h>

char *dtoa19970128(double dd, int mode, int ndigits, int *decpt, int *sign, char **rve);
char *dtoa20161215(double dd, int mode, int ndigits, int *decpt, int *sign, char **rve);
char *dtoa20170421(double dd, int mode, int ndigits, int *decpt, int *sign, char **rve);
char *dtoa20251117(double dd, int mode, int ndigits, int *decpt, int *sign, char **rve);

double strtod19970128(const char*, char**);
double strtod20161215(const char*, char**);
double strtod20170421(const char*, char**);
double strtod20251117(const char*, char**);

static int
roundup(int i, char *p)
{
	if(*p < '5')
		return 0;
	if(*p > '5')
		return 1;
	if((i&1) == 1)
		return 1;
	for(p++; *p != 0; p++)
		if(*p != '0')
			return 1;
	return 0;
}

static void
dmgecvt(char *(*dtoa)(double, int, int, int*, int*, char**), char *buf, double f, int prec)
{
	char *p;
	int exp, neg, ns;

	strcpy(buf+1, dtoa(f, 2, prec, &exp, &neg, 0));
	ns = strlen(buf+1);
	exp--;
	while(ns < prec)
		buf[1+ns++] = '0';
	buf[0] = buf[1];
	buf[1] = '.';
	p = buf+ns+1;
	if(exp != 0) {
		*p++ = 'e';
		if(exp<0) {
			*p++ = '-';
			exp = -exp;
		}
		if(exp >= 100) {
			*p++ = (exp/100)+'0';
			*p++ = (exp/10)%10+'0';
			*p++ = exp%10+'0';
		} else if(exp >= 10) {
			*p++ = (exp/10)+'0';
			*p++ = exp%10+'0';
		} else {
			*p++ = exp+'0';
		}
	}
	*p = '\0';
}

void
loopdmg(int which, char *dst, long long n, double f, int prec)
{
	char buf[100];
	char *(*dtoa)(double, int, int, int*, int*, char**);

	switch(which){
	default:
		abort();
	case 19970128:
		dtoa = dtoa19970128;
		break;
	case 20161215:
		dtoa = dtoa20161215;
		break;
	case 20170421:
		dtoa = dtoa20170421;
		break;
	case 20251117:
		dtoa = dtoa20251117;
		break;
	}

	buf[0] = 0;
	for(long long i = 0; i < n; i++) {
		dmgecvt(dtoa, buf, f, prec);
	}
	strcpy(dst, buf);
}

long long
loopsumdmg(int which, long long n, double *f, int nf, int prec)
{
	char buf[100];
	long long out;
	char *(*dtoa)(double, int, int, int*, int*, char**);

	switch(which){
	default:
		abort();
	case 19970128:
		dtoa = dtoa19970128;
		break;
	case 20161215:
		dtoa = dtoa20161215;
		break;
	case 20170421:
		dtoa = dtoa20170421;
		break;
	case 20251117:
		dtoa = dtoa20251117;
		break;
	}

	for (long long i = 0; i < n; i++) {
		long long total = 0;
		for (int j = 0; j < nf; j++) {
			dmgecvt(dtoa, buf, f[j], prec);
			total += buf[0];
		}
		out = total;
	}
	return out;
}


double
loopdmgstrtod(int which, long long n, char *p)
{
	double d;
	char *e;
	double (*strtod)(const char*, char**);

	switch(which){
	default:
		abort();
	case 19970128:
		strtod = strtod19970128;
		break;
	case 20161215:
		strtod = strtod20161215;
		break;
	case 20170421:
		strtod = strtod20170421;
		break;
	case 20251117:
		strtod = strtod20251117;
		break;
	}


	d = 0.0;
	for (long long i = 0; i < n; i++)
		d = strtod(p, &e);
	return d;
}

double
loopsumdmgstrtod(int which, long long n, char *s)
{
	double f;
	char *e;
	double (*strtod)(const char*, char**);

	switch(which){
	default:
		abort();
	case 19970128:
		strtod = strtod19970128;
		break;
	case 20161215:
		strtod = strtod20161215;
		break;
	case 20170421:
		strtod = strtod20170421;
		break;
	case 20251117:
		strtod = strtod20251117;
		break;
	}


	f = 0.0;
	for (long long i = 0; i < n; i++) {
		char *start = s;
		double total = 0.0;
		for (char *p = s; *p; p++) {
			if(*p == '\n') {
				total += strtod(start, &e);
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

func Loop1997(dst []byte, n int, f float64, prec int) []byte {
	return loop(19970128, dst, n, f, prec)
}

func Loop2016(dst []byte, n int, f float64, prec int) []byte {
	return loop(20161215, dst, n, f, prec)
}

func Loop2017(dst []byte, n int, f float64, prec int) []byte {
	return loop(20170421, dst, n, f, prec)
}

func Loop2025(dst []byte, n int, f float64, prec int) []byte {
	return loop(20251117, dst, n, f, prec)
}

func loop(which int, dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopdmg(C.int(which), (*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func LoopSum1997(n int, fs []float64, prec int) int64 {
	return loopsum(19970128, n, fs, prec)
}

func LoopSum2016(n int, fs []float64, prec int) int64 {
	return loopsum(20161215, n, fs, prec)
}

func LoopSum2017(n int, fs []float64, prec int) int64 {
	return loopsum(20170421, n, fs, prec)
}

func LoopSum2025(n int, fs []float64, prec int) int64 {
	return loopsum(20251117, n, fs, prec)
}

func loopsum(which, n int, fs []float64, prec int) int64 {
	return int64(C.loopsumdmg(C.int(which), C.longlong(n), (*C.double)(&fs[0]), C.int(len(fs)), C.int(prec)))
}

func LoopStrtod1997(n int, s string) float64 {
	return loopstrtod(19970128, n, s)
}

func LoopStrtod2016(n int, s string) float64 {
	return loopstrtod(20161215, n, s)
}

func LoopStrtod2017(n int, s string) float64 {
	return loopstrtod(20170421, n, s)
}

func LoopStrtod2025(n int, s string) float64 {
	return loopstrtod(20251117, n, s)
}

func loopstrtod(which int, n int, src string) float64 {
	p := C.CString(src)
	defer C.free(unsafe.Pointer(p))
	return float64(C.loopdmgstrtod(C.int(which), C.longlong(n), p))
}

func LoopSumStrtod1997(n int, s string) float64 {
	return loopsumstrtod(19970128, n, s)
}

func LoopSumStrtod2016(n int, s string) float64 {
	return loopsumstrtod(20161215, n, s)
}

func LoopSumStrtod2017(n int, s string) float64 {
	return loopsumstrtod(20170421, n, s)
}

func LoopSumStrtod2025(n int, s string) float64 {
	return loopsumstrtod(20251117, n, s)
}

func loopsumstrtod(which int, n int, s string) float64 {
	p := C.CString(s)
	defer C.free(unsafe.Pointer(p))
	return float64(C.loopsumdmgstrtod(C.int(which), C.longlong(n), p))
}
