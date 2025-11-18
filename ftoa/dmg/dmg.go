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

void
loopdmg(int which, char *dst, long long n, double f, int prec)
{
	char buf[100], *p;
	int exp, neg, ns, i;
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

	exp = 0;
	ns = 0;
	buf[0] = 0;
	for(long long i = 0; i < n; i++) {
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
	strcpy(dst, buf);
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
