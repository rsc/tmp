package p

import (
	"runtime"

	"rsc.io/tmp/escbug/q"
)

func P() {
	var buf [24]byte
	runtime.KeepAlive(q.Q(buf[:0], 1))
}
