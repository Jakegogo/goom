//go:build go1.17 && !go1.18 && !arm64 && !amd64
// +build go1.17,!go1.18,!arm64,!amd64

package go117

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

var errUnsupported = errors.New("argdump: PatchFunc not implemented for go1.17 on this architecture")

// DumpFuncImpl stub for unsupported architectures.
type DumpFuncImpl struct{}

// MakeDumpFunc is not available on this architecture.
func MakeDumpFunc(typ reflect.Type) interface{} {
	panic(errUnsupported)
}

// PatchFunc is not available on this architecture.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return nil, errUnsupported
}

// PatchFuncPtr is not available on this architecture.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return nil, errUnsupported
}
