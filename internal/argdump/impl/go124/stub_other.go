//go:build go1.24 && !arm64 && !amd64
// +build go1.24,!arm64,!amd64

package go124

import (
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/impl/shared"
	"github.com/tencent/goom/internal/patch"
)

const goVersion = "go1.24+"

// Debug variables are kept for API compatibility.
var DebugLastProxyFuncValPtr uintptr
var DebugLastProxyCodePtr uintptr
var DebugLastMakeFuncStubPtr uintptr
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr

// DumpFuncImpl stub for unsupported architectures.
type DumpFuncImpl struct{}

// MakeDumpFunc is not available on this architecture.
func MakeDumpFunc(typ reflect.Type) interface{} {
	return shared.StubMakeDumpFunc(goVersion, typ)
}

// PatchFunc is not available on this architecture.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return shared.StubPatchFunc(goVersion, fn)
}

// PatchFuncPtr is not available on this architecture.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return shared.StubPatchFuncPtr(goVersion, originPtr, origFuncVal, typ)
}
