// Package slicemath implements safe “pointer arithmetic” on slices.
// It is a separate package so that packages using it do not need to
// import unsafe directly.
package slicemath

import "unsafe"

// Contains reports whether big contains little;
// that is, it reports whether little is a subslice of big.
func Contains(big, little []byte) bool {
	return uintptr(unsafe.Pointer(&big[0])) <= uintptr(unsafe.Pointer(&little[0])) &&
		uintptr(unsafe.Pointer(&little[len(little)-1])) <= uintptr(unsafe.Pointer(&big[len(big)-1]))
}

// Offset reports little's starting position within big.
// The caller must have checked sliceContains(big, little) already.
func Offset(big, little []byte) uintptr {
	return uintptr(unsafe.Pointer(&little[0])) - uintptr(unsafe.Pointer(&big[0]))
}
