//go:build gcvt

#include <stdio.h>

char*
gcvt(double f, int prec, char *buf)
{
	sprintf(buf, "%.*g", prec, f);
	return buf;
}
