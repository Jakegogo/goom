//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

package go118

import (
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/impl/regabi"
	"github.com/tencent/goom/internal/patch"
)

// Re-export regabi types for go1.18 arm64
type DumpFuncImpl = regabi.DumpFuncImpl

// MakeDumpFunc creates a dump function for the given type.
// This is a thin wrapper around regabi.MakeDumpFunc for backward compatibility.
func MakeDumpFunc(typ reflect.Type) interface{} {
	return regabi.MakeDumpFunc(typ)
}

// PatchFunc patches a function to dump its arguments.
// This is a thin wrapper around regabi.PatchFunc for backward compatibility.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return regabi.PatchFunc(fn)
}

// PatchFuncPtr patches a function by its pointer.
// This is a thin wrapper around regabi.PatchFuncPtr for backward compatibility.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return regabi.PatchFuncPtr(originPtr, origFuncVal, typ)
}
