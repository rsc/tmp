// +build ignore

package p

import "math"

//goo:fix
func f() {
}

func h() {
	f := 1.0
	f = math.Minus(f)
	println(f)
}

func g() {
	f()
	h()
}
