package rsc

/*
#include <string.h>
#include <stdio.h>

void rscdtoa(double f, char *s, int *exp, int *neg, int *ns);

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
looprsc(char *dst, long long n, double f, int prec)
{
	char buf[100], *p;
	int exp, neg, ns, i;

	exp = 0;
	ns = 0;
	buf[0] = 0;
	for(long long i = 0; i < n; i++) {
		rscdtoa(f, buf+1, &exp, &neg, &ns);
		if(ns > prec) {
			if(roundup(buf[1+prec-1], buf+1+prec)) {
				 i = 1+prec-1;
				 for(; i >= 1; i--) {
				 	buf[i]++;
				 	if(buf[i] <= '9') {
				 		break;
				 	}
				 	buf[i] = '0';
				 }
				 if(i < 1) {
				 	buf[1] = '1';
				 	exp++;
				 }
			}
			exp += ns-prec;
			ns = prec;
		}
		while(ns < prec) {
			buf[1+ns++] = '0';
			exp--;
		}
		exp += ns-1;
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
	C.looprsc((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}
