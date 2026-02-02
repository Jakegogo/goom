//go:build go1.13 && !go1.17 && 386
// +build go1.13,!go1.17,386

package argdump

import "unsafe"

func callOriginalPreGo117(impl *dumpFuncImpl, frame unsafe.Pointer) {
	impl.hookGuard.UnpatchWithLock()
	defer impl.hookGuard.Restore()
	runtimeReflectcall(nil, impl.origFuncVal, frame, impl.stackArgsSz+uint32(ptrSize), impl.stackRetOff)
}
