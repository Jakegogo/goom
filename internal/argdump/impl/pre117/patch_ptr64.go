//go:build go1.13 && !go1.17 && (amd64 || arm64)
// +build go1.13
// +build !go1.17
// +build amd64 arm64

package pre117

import (
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/patch"
)

func patchEntry(originPtr uintptr, ctxtPtr uintptr, makeFuncStubPtr uintptr, impl *DumpFuncImpl) (*patch.Guard, error) {
	if impl.UseTrampoline {
		guard, err := patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubPtr)
		if err == nil && guard.FixOriginFunc() != 0 {
			// Build a minimal funcval for the trampoline entry: {code, ctxt=nil}.
			type funcval struct {
				fn   uintptr
				ctxt uintptr
			}
			fv := &funcval{fn: guard.FixOriginFunc()}
			impl.OrigFuncValBox = fv
			impl.OrigFuncVal = unsafe.Pointer(fv)
			return guard, nil
		}
		impl.UseTrampoline = false
	}
	_ = abi.PtrSize // ensure import
	return patch.PtrCodeWithCtx(originPtr, ctxtPtr, makeFuncStubPtr, 0)
}
