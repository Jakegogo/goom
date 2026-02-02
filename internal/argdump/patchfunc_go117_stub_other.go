//go:build go1.17 && !go1.18 && !arm64
// +build go1.17,!go1.18,!arm64

package argdump

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

// Debug variables are kept for API compatibility with the go1.24+ implementation.
// They are populated only on implementations that support PatchFunc.
var DebugLastProxyFuncValPtr uintptr
var DebugLastProxyCodePtr uintptr
var DebugLastMakeFuncStubPtr uintptr
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr

func PatchFunc(fn interface{}) (*patch.Guard, error) {
	_ = fn
	return nil, errors.New("argdump: PatchFunc is not implemented on this architecture for go1.17 yet (see COMPATIBILITY.md)")
}

func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	_ = originPtr
	_ = origFuncVal
	_ = typ
	return nil, errors.New("argdump: PatchFuncPtr is not implemented on this architecture for go1.17 yet (see COMPATIBILITY.md)")
}
