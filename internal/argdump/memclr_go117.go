//go:build go1.17
// +build go1.17

package argdump

import "unsafe"

func memclr(p unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	ni := int(n)
	if uintptr(ni) != n {
		panic("argdump: memclr size overflow")
	}
	b := unsafe.Slice((*byte)(p), ni)
	for i := range b {
		b[i] = 0
	}
}
