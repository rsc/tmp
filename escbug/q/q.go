package q

func nop(x int) int { return x }

func Q(dst []byte, x int64) []byte {
	return q(dst, x)
}

func q[T int32 | int64](dst []byte, x T) []byte {
	return append(dst, byte(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(nop(int(x))))))))))))))))))))))))
}
