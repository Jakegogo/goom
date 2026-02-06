//go:build go1.13 && !go1.17 && !amd64 && !arm64 && !386
// +build go1.13,!go1.17,!amd64,!arm64,!386

package pre117

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

var errUnsupported = errors.New("argdump: unsupported architecture for go1.13-go1.16")

// MakeDumpFunc is not available on unsupported architectures.
func MakeDumpFunc(typ reflect.Type) interface{} {
	panic(errUnsupported)
}

// PatchFunc is not available on unsupported architectures.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return nil, errUnsupported
}

// PatchFuncPtr is not available on unsupported architectures.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return nil, errUnsupported
}

// CallOriginal is not available on unsupported architectures.
func CallOriginal(impl *DumpFuncImpl, frame unsafe.Pointer) {
	panic(errUnsupported)
}
