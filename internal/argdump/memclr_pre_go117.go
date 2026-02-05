//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

import "unsafe"

func memclr(p unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(uintptr(p) + i)) = 0
	}
}
