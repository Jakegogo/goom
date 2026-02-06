//go:build go1.13 && !go1.17 && (amd64 || arm64)
// +build go1.13
// +build !go1.17
// +build amd64 arm64

package pre117

import (
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
)

// CallOriginal calls the original function for 64-bit architectures.
func CallOriginal(impl *DumpFuncImpl, frame unsafe.Pointer) {
	if impl.UseTrampoline {
		runtimeReflectcall(nil, impl.OrigFuncVal, frame, impl.StackArgsSz+uint32(abi.PtrSize), impl.StackRetOff)
		return
	}

	impl.HookGuard.UnpatchWithLock()
	defer impl.HookGuard.Restore()
	runtimeReflectcall(nil, impl.OrigFuncVal, frame, impl.StackArgsSz+uint32(abi.PtrSize), impl.StackRetOff)
}
