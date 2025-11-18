package dblconv

/*
int loopdblconv(char*, long long, double, int);
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
