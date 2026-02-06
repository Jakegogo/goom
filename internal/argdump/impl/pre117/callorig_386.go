//go:build go1.13 && !go1.17 && 386
// +build go1.13,!go1.17,386

package pre117

import (
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
)

// CallOriginal calls the original function for 386 architecture.
func CallOriginal(impl *DumpFuncImpl, frame unsafe.Pointer) {
	impl.HookGuard.UnpatchWithLock()
	defer impl.HookGuard.Restore()
	runtimeReflectcall(nil, impl.OrigFuncVal, frame, impl.StackArgsSz+uint32(abi.PtrSize), impl.StackRetOff)
}
