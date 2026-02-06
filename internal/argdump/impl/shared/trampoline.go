//go:build go1.17 && (amd64 || arm64)
// +build go1.17
// +build amd64 arm64

package shared

import "unsafe"

// CallReflectTrampolineHolder is a placeholder function whose code will be overwritten
// with the fixed origin instructions by patch.PtrTrampoline.
//
//go:noinline
func CallReflectTrampolineHolder(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	// Keep this function "large" so the trampoline copier has room.
	// This code will never execute once patched.
	var x uintptr
	x ^= uintptr(unsafe.Pointer(ctxt))
	x ^= uintptr(unsafe.Pointer(frame))
	if retValid != nil && *retValid {
		x++
	}
	if regs != nil {
		x ^= uintptr(regs)
	}
	for i := 0; i < 64; i++ {
		x ^= uintptr(i) * 0x9e3779b97f4a7c15
	}
	if x == 0xdeadbeef {
		panic("unreachable")
	}
}
