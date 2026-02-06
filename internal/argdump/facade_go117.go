//go:build go1.17 && !go1.18
// +build go1.17,!go1.18

package argdump

import (
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/impl/go117"
	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/patch"
)

// Re-export flags from shared package.
var (
	DebugEnabled                = &flags.DebugEnabled
	DumpEnabled                 = &flags.DumpEnabled
	DebugCallDumpPtr            = &flags.DebugCallDumpPtr
	DebugCallReflectPtr         = &flags.DebugCallReflectPtr
	DebugRuntimeReflectcallPtr  = &flags.DebugRuntimeReflectcallPtr
	DebugRuntimeSpillArgsPtr    = &flags.DebugRuntimeSpillArgsPtr
	DebugRuntimeUnspillArgsPtr  = &flags.DebugRuntimeUnspillArgsPtr
	DebugMoveMakeFuncArgPtrsPtr = &flags.DebugMoveMakeFuncArgPtrsPtr
	DebugLastProxyFuncValPtr    = &flags.DebugLastProxyFuncValPtr
	DebugLastProxyCodePtr       = &flags.DebugLastProxyCodePtr
	DebugLastMakeFuncStubPtr    = &flags.DebugLastMakeFuncStubPtr
	DebugOrigFuncValPtr         = &flags.DebugOrigFuncValPtr
	DebugOrigCodePtr            = &flags.DebugOrigCodePtr
)

// MakeDumpFunc returns a function value of the provided function type.
func MakeDumpFunc(typ reflect.Type) interface{} {
	return go117.MakeDumpFunc(typ)
}

// PatchFunc patches the provided function so that every call will print all arguments.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	return go117.PatchFunc(fn)
}

// PatchFuncPtr is PatchFunc but takes an explicit function entry address and function type.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	return go117.PatchFuncPtr(originPtr, origFuncVal, typ)
}
