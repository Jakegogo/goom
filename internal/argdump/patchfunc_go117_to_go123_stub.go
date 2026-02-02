//go:build go1.18 && !go1.24 && !arm64
// +build go1.18,!go1.24,!arm64

package argdump

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

// Debug variables are kept for API compatibility with the go1.24+ implementation.
// They are populated only in the go1.24+ PatchFunc path.
var DebugLastProxyFuncValPtr uintptr
var DebugLastProxyCodePtr uintptr
var DebugLastMakeFuncStubPtr uintptr
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr

// PatchFunc is not implemented for go1.18-go1.23 yet.
//
// It is an *advanced* feature (entry patch + reflect.makeFuncStub + patched reflect.callReflect),
// and requires careful per-version ABI/stack-map validation.
// See `COMPATIBILITY.md`.
//
// Why we *do* support go1.17/arm64 separately:
//   - go1.17 makeFuncStub path is effectively stack-only (regs=nil) and we have a dedicated, tested implementation.
//   - go1.18+ makeFuncStub passes an abi.RegArgs pointer and has different spill/unspill behavior, so we gate it
//     until it is validated end-to-end for PatchFunc.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	_ = fn
	return nil, errors.New("argdump: PatchFunc is not implemented for this Go version yet")
}

// PatchFuncPtr is the go1.17-go1.23 stub for the go1.24+ implementation.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	_ = originPtr
	_ = origFuncVal
	_ = typ
	return nil, errors.New("argdump: PatchFuncPtr is not implemented for this Go version yet")
}
