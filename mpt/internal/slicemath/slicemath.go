// Package slicemath implements safe “pointer arithmetic” on slices.
// It is a separate package so that packages using it do not need to
// import unsafe directly.
package slicemath

import "unsafe"

// contains reports whether big contains little;
// that is, it reports whether little is a subslice of big.
func contains(big, little []byte) bool {
	return uintptr(unsafe.Pointer(&big[0])) <= uintptr(unsafe.Pointer(&little[0])) &&
		uintptr(unsafe.Pointer(&little[len(little)-1])) <= uintptr(unsafe.Pointer(&big[len(big)-1]))
}

// Offset reports little's starting position within big.
// If big does not contain little, Offset returns ^uintptr(0), false.
// The caller must have checked sliceContains(big, little) already.
func Offset(big, little []byte) (offset uintptr, ok bool) {
	if !contains(big, little) {
		return ^uintptr(0), false
	}
	return uintptr(unsafe.Pointer(&little[0])) - uintptr(unsafe.Pointer(&big[0])), true
}
