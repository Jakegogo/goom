//go:build go1.13 && !go1.17 && !amd64 && !arm64 && !386
// +build go1.13,!go1.17,!amd64,!arm64,!386

package argdump

import (
	"errors"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

var errPreGo117 = errors.New("argdump: unsupported Go version (requires go1.17+)")

// MakeDumpFunc is not available on go1.13-go1.16 in this port yet.
func MakeDumpFunc(typ reflect.Type) interface{} {
	panic(errPreGo117)
}

// PatchFunc is not available on go1.13-go1.16 in this port yet.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return nil, errPreGo117
}

// PatchFuncPtr is not available on go1.13-go1.16 in this port yet.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return nil, errPreGo117
}
