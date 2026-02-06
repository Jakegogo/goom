//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package mem

import "unsafe"

// Memclr clears n bytes starting at p.
func Memclr(p unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(uintptr(p) + i)) = 0
	}
}
