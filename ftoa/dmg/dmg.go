package dmg

import "C"

/*
#include <string.h>

char *dmgdtoa(double dd, int mode, int ndigits, int *decpt, int *sign, char **rve);

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
loopdmg(char *dst, long long n, double f, int prec)
{
	char buf[100], *p;
	int exp, neg, ns, i;

	exp = 0;
	ns = 0;
	buf[0] = 0;
	for(long long i = 0; i < n; i++) {
		strcpy(buf+1, dmgdtoa(f, 2, prec, &exp, &neg, 0));
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
*/
import "C"
import "unsafe"

func Loop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopdmg((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}
