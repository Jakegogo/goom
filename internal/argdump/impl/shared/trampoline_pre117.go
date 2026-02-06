//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package shared

import "unsafe"

// CallReflectTrampolineHolderPreGo117 is a placeholder function whose code will be overwritten
// with the fixed origin instructions by patch.PtrTrampoline.
//
//go:noinline
func CallReflectTrampolineHolderPreGo117(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool) {
	var x uintptr
	x ^= uintptr(unsafe.Pointer(ctxt))
	x ^= uintptr(unsafe.Pointer(frame))
	if retValid != nil && *retValid {
		x++
	}
	for i := 0; i < 64; i++ {
		// Keep constants within 32-bit uintptr range (also fine on 64-bit).
		x ^= uintptr(i) * uintptr(0x9e3779b9)
	}
	if x == 0xdeadbeef {
		panic("unreachable")
	}
}
