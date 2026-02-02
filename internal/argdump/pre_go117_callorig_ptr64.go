//go:build go1.13 && !go1.17 && (amd64 || arm64)
// +build go1.13
// +build !go1.17
// +build amd64 arm64

package argdump

import "unsafe"

func callOriginalPreGo117(impl *dumpFuncImpl, frame unsafe.Pointer) {
	if impl.useTrampoline {
		runtimeReflectcall(nil, impl.origFuncVal, frame, impl.stackArgsSz+uint32(ptrSize), impl.stackRetOff)
		return
	}

	impl.hookGuard.UnpatchWithLock()
	defer impl.hookGuard.Restore()
	runtimeReflectcall(nil, impl.origFuncVal, frame, impl.stackArgsSz+uint32(ptrSize), impl.stackRetOff)
}
