//go:build go1.17
// +build go1.17

package mem

import "unsafe"

// Memclr clears n bytes starting at p.
func Memclr(p unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	ni := int(n)
	if uintptr(ni) != n {
		panic("mem: memclr size overflow")
	}
	b := unsafe.Slice((*byte)(p), ni)
	for i := range b {
		b[i] = 0
	}
}
