// Copied and modified from https://www.netlib.org/fp/p9fmt.tgz

/*
 * The authors of this software are Rob Pike and Ken Thompson.
 *              Copyright (c) 2002 by Lucent Technologies.
 * Permission to use, copy, modify, and distribute this software for any
 * purpose without fee is hereby granted, provided that this entire notice
 * is included in all copies of any software which is or includes a copy
 * or modification of this software and in all copies of the supporting
 * documentation for such software.
 * THIS SOFTWARE IS BEING PROVIDED "AS IS", WITHOUT ANY EXPRESS OR IMPLIED
 * WARRANTY.  IN PARTICULAR, NEITHER THE AUTHORS NOR LUCENT TECHNOLOGIES MAKE ANY
 * REPRESENTATION OR WARRANTY OF ANY KIND CONCERNING THE MERCHANTABILITY
 * OF THIS SOFTWARE OR ITS FITNESS FOR ANY PARTICULAR PURPOSE.
 */
#include <stdio.h>
#include <math.h>
#include <float.h>
#include <string.h>
#include <stdlib.h>
#include <errno.h>
#include <stdarg.h>

#define nil ((void*)0)

#define maxFloat64 1.79769313486231570814527423731704356798070e+308

enum {
	FmtSharp = 1,
	FmtSign = 2,
	FmtSpace = 4,
};

extern double fmtstrtod(const char*, char**);

enum
{
	FDEFLT	= 6,
	NSIGNIF	= 17
};

/*
 * first few powers of 10, enough for about 1/2 of the
 * total space for doubles.
 */
static double pows10[] =
{
	  1e0,   1e1,   1e2,   1e3,   1e4,   1e5,   1e6,   1e7,   1e8,   1e9,
	 1e10,  1e11,  1e12,  1e13,  1e14,  1e15,  1e16,  1e17,  1e18,  1e19,
	 1e20,  1e21,  1e22,  1e23,  1e24,  1e25,  1e26,  1e27,  1e28,  1e29,
	 1e30,  1e31,  1e32,  1e33,  1e34,  1e35,  1e36,  1e37,  1e38,  1e39,
	 1e40,  1e41,  1e42,  1e43,  1e44,  1e45,  1e46,  1e47,  1e48,  1e49,
	 1e50,  1e51,  1e52,  1e53,  1e54,  1e55,  1e56,  1e57,  1e58,  1e59,
	 1e60,  1e61,  1e62,  1e63,  1e64,  1e65,  1e66,  1e67,  1e68,  1e69,
	 1e70,  1e71,  1e72,  1e73,  1e74,  1e75,  1e76,  1e77,  1e78,  1e79,
	 1e80,  1e81,  1e82,  1e83,  1e84,  1e85,  1e86,  1e87,  1e88,  1e89,
	 1e90,  1e91,  1e92,  1e93,  1e94,  1e95,  1e96,  1e97,  1e98,  1e99,
	1e100, 1e101, 1e102, 1e103, 1e104, 1e105, 1e106, 1e107, 1e108, 1e109,
	1e110, 1e111, 1e112, 1e113, 1e114, 1e115, 1e116, 1e117, 1e118, 1e119,
	1e120, 1e121, 1e122, 1e123, 1e124, 1e125, 1e126, 1e127, 1e128, 1e129,
	1e130, 1e131, 1e132, 1e133, 1e134, 1e135, 1e136, 1e137, 1e138, 1e139,
	1e140, 1e141, 1e142, 1e143, 1e144, 1e145, 1e146, 1e147, 1e148, 1e149,
	1e150, 1e151, 1e152, 1e153, 1e154, 1e155, 1e156, 1e157, 1e158, 1e159,
};

#define	pow10(x)  fmtpow10(x)

static double
pow10(int n)
{
	double d;
	int neg;

	neg = 0;
	if(n < 0){
		if(n < DBL_MIN_10_EXP){
			return 0.;
		}
		neg = 1;
		n = -n;
	}else if(n > DBL_MAX_10_EXP){
		return HUGE_VAL;
	}
	if(n < sizeof(pows10)/sizeof(pows10[0]))
		d = pows10[n];
	else{
		d = pows10[sizeof(pows10)/sizeof(pows10[0]) - 1];
		for(;;){
			n -= sizeof(pows10)/sizeof(pows10[0]) - 1;
			if(n < sizeof(pows10)/sizeof(pows10[0])){
				d *= pows10[n];
				break;
			}
			d *= pows10[sizeof(pows10)/sizeof(pows10[0]) - 1];
		}
	}
	if(neg){
		return 1./d;
	}
	return d;
}

static int
xadd(char *a, int n, int v)
{
	char *b;
	int c;

	if(n < 0 || n >= NSIGNIF)
		return 0;
	for(b = a+n; b >= a; b--) {
		c = *b + v;
		if(c <= '9') {
			*b = c;
			return 0;
		}
		*b = '0';
		v = 1;
	}
	*a = '1';	/* overflow adding */
	return 1;
}

static int
xsub(char *a, int n, int v)
{
	char *b;
	int c;

	for(b = a+n; b >= a; b--) {
		c = *b - v;
		if(c >= '0') {
			*b = c;
			return 0;
		}
		*b = '9';
		v = 1;
	}
	*a = '9';	/* underflow subtracting */
	return 1;
}

static void
xaddexp(char *p, int e)
{
	char se[9];
	int i;

	*p++ = 'e';
	if(e < 0) {
		*p++ = '-';
		e = -e;
	}
	i = 0;
	while(e) {
		se[i++] = e % 10 + '0';
		e /= 10;
	}
	if(i == 0) {
		*p++ = '0';
	} else {
		while(i > 0)
			*p++ = se[--i];
	}
	*p++ = '\0';
}

static char*
xdodtoa(char *s1, double f, int chr, int prec, int *decpt, int *rsign)
{
	char s2[NSIGNIF+10];
	double g, h;
	int e, d, i;
	int c2, sign, oerr;

	if(chr == 'F')
		chr = 'f';
	if(prec > NSIGNIF)
		prec = NSIGNIF;
	if(prec < 0)
		prec = 0;
	if(f != f) {
		*decpt = 9999;
		*rsign = 0;
		strcpy(s1, "nan");
		return &s1[3];
	}
	sign = 0;
	if(f < 0) {
		f = -f;
		sign++;
	}
	*rsign = sign;
	if(f > maxFloat64 || f < -maxFloat64) {
		*decpt = 9999;
		strcpy(s1, "inf");
		return &s1[3];
	}

	e = 0;
	g = f;
	if(g != 0) {
		frexp(f, &e);
		e = e * .301029995664;
		if(e >= -150 && e <= +150) {
			d = 0;
			h = f;
		} else {
			d = e/2;
			h = f * pow10(-d);
		}
		g = h * pow10(d-e);
		while(g < 1) {
			e--;
			g = h * pow10(d-e);
		}
		while(g >= 10) {
			e++;
			g = h * pow10(d-e);
		}
	}

	/*
	 * convert NSIGNIF digits and convert
	 * back to get accuracy.
	 */
	for(i=0; i<NSIGNIF; i++) {
		d = g;
		s1[i] = d + '0';
		g = (g - d) * 10;
	}
	s1[i] = 0;

	/*
	 * try decimal rounding to eliminate 9s
	 */
	c2 = prec + 1;
	if(chr == 'f')
		c2 += e;
	oerr = errno;
	if(c2 >= NSIGNIF-2) {
		strcpy(s2, s1);
		d = e;
		s1[NSIGNIF-2] = '0';
		s1[NSIGNIF-1] = '0';
		xaddexp(s1+NSIGNIF, e-NSIGNIF+1);
		g = fmtstrtod(s1, nil);
		if(g == f)
			goto found;
		if(xadd(s1, NSIGNIF-3, 1)) {
			e++;
			xaddexp(s1+NSIGNIF, e-NSIGNIF+1);
		}
		g = fmtstrtod(s1, nil);
		if(g == f)
			goto found;
		strcpy(s1, s2);
		e = d;
	}

	/*
	 * convert back so s1 gets exact answer
	 */
	for(d = 0; d < 10; d++) {
		xaddexp(s1+NSIGNIF, e-NSIGNIF+1);
		g = fmtstrtod(s1, nil);
		if(f > g) {
			if(xadd(s1, NSIGNIF-1, 1))
				e--;
			continue;
		}
		if(f < g) {
			if(xsub(s1, NSIGNIF-1, 1))
				e++;
			continue;
		}
		break;
	}

found:
	errno = oerr;

	/*
	 * sign
	 */
	d = 0;
	i = 0;

	/*
	 * round & adjust 'f' digits
	 */
	c2 = prec + 1;
	if(chr == 'f'){
		if(xadd(s1, c2+e, 5))
			e++;
		c2 += e;
		if(c2 < 0){
			c2 = 0;
			e = -prec - 1;
		}
	}else{
		if(xadd(s1, c2, 5))
			e++;
	}
	if(c2 > NSIGNIF){
		c2 = NSIGNIF;
	}

	*decpt = e + 1;

	/*
	 * terminate the converted digits
	 */
	s1[c2] = '\0';
	return &s1[c2];
}

/*
 * this function works like the standard dtoa
 */
char*
kendtoa(double f, int mode, int ndigits, int *decpt, int *rsign, char **rve)
{
	static char s2[NSIGNIF + 10];
	char *es;
	int chr, prec;

	switch(mode) {
	/* like 'e' */
	case 2:
	case 4:
	case 6:
	case 8:
		chr = 'e';
		break;
	/* like 'g' */
	case 0:
	case 1:
	default:
		chr = 'g';
		break;
	/* like 'f' */
	case 3:
	case 5:
	case 7:
	case 9:
		chr = 'f';
		break;
	}

	if(chr != 'f' && ndigits){
		ndigits--;
	}
	prec = ndigits;
	if(prec > NSIGNIF)
		prec = NSIGNIF;
	if(ndigits == 0)
		prec = NSIGNIF;
	es = xdodtoa(s2, f, chr, prec, decpt, rsign);

	/*
	 * strip trailing 0
	 */
	for(; es > s2 + 1; es--){
		if(es[-1] != '0'){
			break;
		}
	}
	*es = '\0';
	if(rve != NULL)
		*rve = es;
	return s2;
}
