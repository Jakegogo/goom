//go:build go1.24 && !arm64 && !amd64
// +build go1.24,!arm64,!amd64

package go124

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

// Debug variables are kept for API compatibility with the arm64 implementation.
var DebugLastProxyFuncValPtr uintptr
var DebugLastProxyCodePtr uintptr
var DebugLastMakeFuncStubPtr uintptr
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr

// DumpFuncImpl stub for unsupported architectures.
type DumpFuncImpl struct{}

// MakeDumpFunc is not available on this architecture.
func MakeDumpFunc(typ reflect.Type) interface{} {
	panic(errors.New("argdump: MakeDumpFunc is not implemented for go1.24+ on this architecture"))
}

// PatchFunc is not implemented for go1.24+ on non-arm64 yet.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	_ = fn
	return nil, errors.New("argdump: PatchFunc is not implemented for this Go version/architecture yet")
}

// PatchFuncPtr is the go1.24+ stub for non-arm64 architectures.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	_ = originPtr
	_ = origFuncVal
	_ = typ
	return nil, errors.New("argdump: PatchFuncPtr is not implemented for this Go version/architecture yet")
}
