//go:build go1.13 && !go1.17 && 386
// +build go1.13,!go1.17,386

package argdump

import "github.com/tencent/goom/internal/patch"

func patchPreGo117Entry(originPtr uintptr, ctxtPtr uintptr, makeFuncStubPtr uintptr, impl *dumpFuncImpl) (*patch.Guard, error) {
	// 386: no trampoline support in patch layer.
	impl.useTrampoline = false
	return patch.PtrCodeWithCtx(originPtr, ctxtPtr, makeFuncStubPtr, 0)
}


