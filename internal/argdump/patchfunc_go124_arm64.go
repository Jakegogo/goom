//go:build go1.24
// +build go1.24

package argdump

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// DebugLastProxyFuncValPtr is the last proxy func value pointer built by PatchFuncPtr.
// It is intended for diagnostics/testing only.
var DebugLastProxyFuncValPtr uintptr

// DebugLastProxyCodePtr is the first word of DebugLastProxyFuncValPtr (the code pointer).
var DebugLastProxyCodePtr uintptr

// DebugLastMakeFuncStubPtr is the resolved code pointer used for reflect.makeFuncStub.
var DebugLastMakeFuncStubPtr uintptr

// DebugOrigFuncValPtr/DebugOrigCodePtr capture the original function value pointer and its code pointer.
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr

// guardKeepAlive retains patch-specific contexts (like *dumpFuncImpl) so the GC
// doesn't reclaim them while patched code still references their addresses.
//
// Without this, the patched function may still jump with x26=implPtr, but implPtr
// is not visible to the GC (it's embedded as an immediate in text), leading to
// "found pointer to free object"/"marked free object" crashes under GC.
var guardKeepAlive sync.Map // map[*patch.Guard]any

// PatchFunc patches the provided function so that every call will:
// - print all arguments using EncodeJSONFromAddr
// - then continue executing the original function
// - and preserve the original return values
//
// Note: this relies on patching the function entry point, so it is inherently unsafe
// and should only be used in controlled environments (tests / debugging).
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	if fn == nil {
		return nil, errors.New("argdump: PatchFunc nil")
	}
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("argdump: PatchFunc expects func, got %s", t.Kind())
	}
	v := reflect.ValueOf(fn)
	originPtr := v.Pointer()
	// Note: don't attempt to auto-resolve an "inner" function for arbitrary non-wrapper funcs.
	// Heuristic inner scanning is only safe for known wrapper patterns.
	origFuncVal := bytecode.GetPtr(v)
	DebugOrigFuncValPtr = uintptr(origFuncVal)
	DebugOrigCodePtr = *(*uintptr)(origFuncVal)
	return PatchFuncPtr(originPtr, origFuncVal, t)
}

// PatchFuncPtr is PatchFunc but takes an explicit function entry address and function type.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	if originPtr == 0 {
		return nil, errors.New("argdump: PatchFuncPtr originPtr=0")
	}
	if origFuncVal == nil {
		return nil, errors.New("argdump: PatchFuncPtr origFuncValPtr=0")
	}
	if typ == nil || typ.Kind() != reflect.Func {
		return nil, errors.New("argdump: PatchFuncPtr typ must be a func type")
	}

	ensureCallReflectPatched()

	makeFuncStubPtr, err := unexports2.FindFuncByName("reflect.makeFuncStub")
	if err != nil {
		return nil, err
	}

	abid := newAbiDescFromFuncType(typ)
	impl := &dumpFuncImpl{
		makeFuncCtxt: makeFuncCtxt{
			fn:      makeFuncStubPtr,
			stack:   abid.stackPtrs,
			argLen:  abid.stackCallArgsSize,
			regPtrs: abid.inRegPtrs,
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	impl.origFuncVal = origFuncVal
	impl.stackArgsSz = uint32(abid.frameTypeSizeBytes())
	impl.stackRetOff = uint32(abid.retOffset)
	impl.frameSize = uint32(abid.frameSizeBytes())
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr impl=%p fn=0x%x firstWord=0x%x\n",
			impl, impl.makeFuncCtxt.fn, *(*uintptr)(unsafe.Pointer(impl)),
		)
	}

	// Patch origin entry to call reflect.makeFuncStub, with the architecture's context register
	// pointing at our makeFuncCtxt
	// (dumpFuncImpl embeds it at the start). The call stub preserves the caller's x26.
	ctxtPtr := uintptr(unsafe.Pointer(impl))
	// Debug aid: exported for tests/diagnostics.
	DebugLastProxyFuncValPtr = ctxtPtr
	DebugLastProxyCodePtr = makeFuncStubPtr
	DebugLastMakeFuncStubPtr = makeFuncStubPtr
	// If the original function value has non-nil context, trampoline-call is not safe
	// because we can't synthesize an equivalent closure object for the trampoline.
	// In that case we fall back to Unpatch/Restore around runtime.reflectcall.
	// Heuristic: closures have compiler-generated names containing ".func".
	// (They require a context pointer; trampoline-calling can't synthesize that reliably.)
	fnName := ""
	if f := runtime.FuncForPC(originPtr); f != nil {
		fnName = f.Name()
	}
	impl.useTrampoline = fnName != "" && !strings.Contains(fnName, ".func")

	var guard *patch.Guard
	if impl.useTrampoline {
		guard, err = patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubPtr)
		if err == nil && guard.FixOriginFunc() != 0 {
			// Call original via the trampoline entry by using a minimal funcval:
			// first word is code pointer, second word is ctxt (nil).
			type funcval struct {
				fn   uintptr
				ctxt uintptr
			}
			fv := &funcval{fn: guard.FixOriginFunc()}
			impl.origFuncValBox = fv
			impl.origFuncVal = unsafe.Pointer(fv)
		} else {
			// Fallback: some functions have control-flow that jumps into the overwritten prologue,
			// which can't be safely trampoline-called without a larger rewrite. In that case,
			// we fall back to Unpatch/Restore around runtime.reflectcall.
			impl.useTrampoline = false
			guard, err = patch.PtrCodeWithCtx(originPtr, ctxtPtr, makeFuncStubPtr, 0)
			if err != nil {
				return nil, err
			}
		}
	} else {
		guard, err = patch.PtrCodeWithCtx(originPtr, ctxtPtr, makeFuncStubPtr, 0)
		if err != nil {
			return nil, err
		}
	}
	impl.hookGuard = guard
	guardKeepAlive.Store(guard, impl)
	runtime.SetFinalizer(guard, func(g *patch.Guard) {
		guardKeepAlive.Delete(g)
	})
	return guard, nil
}
