package main

import "fmt"

func f() uint64
func cpuid(eaxArg, ecxArg uint32) (eax, ebx, ecx, edx uint32)

func main() {
	_, ebx, _, _ := cpuid(7, 0)
	ff := f()
	pass := "CORRECT"
	if ff != 1 {
		pass = "WRONG"
	}
	fmt.Printf("bmi2 %v f %#x %s\n", ebx&0x100 != 0, ff, pass)
}
