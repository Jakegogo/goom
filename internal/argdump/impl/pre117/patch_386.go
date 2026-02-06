//go:build go1.13 && !go1.17 && 386
// +build go1.13,!go1.17,386

package pre117

import "github.com/tencent/goom/internal/patch"

func patchEntry(originPtr uintptr, ctxtPtr uintptr, makeFuncStubPtr uintptr, impl *DumpFuncImpl) (*patch.Guard, error) {
	// 386: no trampoline support in patch layer.
	impl.UseTrampoline = false
	return patch.PtrCodeWithCtx(originPtr, ctxtPtr, makeFuncStubPtr, 0)
}
