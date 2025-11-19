package dblconv

/*
int loopdblconv(char*, long long, double, int);
long long loopsumdblconv(long long n, double *f, int nf, int prec);
*/
import "C"
import "unsafe"

func Loop(dst []byte, n int, f float64, prec int) []byte {
	var buf [100]byte
	C.loopdblconv((*C.char)(unsafe.Pointer(&buf[0])), C.longlong(n), C.double(f), C.int(prec))
	i := 0
	for i < len(buf) && buf[i] != 0 {
		i++
	}
	return append(dst, buf[:i]...)
}

func LoopSum(n int, fs []float64, prec int) int64 {
	return int64(C.loopsumdblconv(C.longlong(n), (*C.double)(&fs[0]), C.int(len(fs)), C.int(prec)))
}
