package shared

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/patch"
)

// UnsupportedArchError creates an error indicating the operation is not supported on this architecture.
func UnsupportedArchError(goVersion, operation string) error {
	return fmt.Errorf("argdump: %s is not implemented for %s on this architecture", operation, goVersion)
}

// StubMakeDumpFunc is a stub implementation of MakeDumpFunc for unsupported architectures.
func StubMakeDumpFunc(goVersion string, typ reflect.Type) interface{} {
	panic(UnsupportedArchError(goVersion, "MakeDumpFunc"))
}

// StubPatchFunc is a stub implementation of PatchFunc for unsupported architectures.
func StubPatchFunc(goVersion string, fn interface{}) (*patch.Guard, error) {
	_ = fn
	return nil, UnsupportedArchError(goVersion, "PatchFunc")
}

// StubPatchFuncPtr is a stub implementation of PatchFuncPtr for unsupported architectures.
func StubPatchFuncPtr(goVersion string, originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	_ = originPtr
	_ = origFuncVal
	_ = typ
	return nil, UnsupportedArchError(goVersion, "PatchFuncPtr")
}

// DebugVars holds debug variables for API compatibility.
type DebugVars struct {
	LastProxyFuncValPtr  uintptr
	LastProxyCodePtr     uintptr
	LastMakeFuncStubPtr  uintptr
	OrigFuncValPtr       uintptr
	OrigCodePtr          uintptr
}

// NewDebugVars creates a new DebugVars instance.
func NewDebugVars() *DebugVars {
	return &DebugVars{}
}
