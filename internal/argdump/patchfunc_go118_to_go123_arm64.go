//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

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

func makeFuncStubEntry()

// go1.18-go1.23 (arm64) PatchFunc implementation.
//
// Key stability decisions:
// - Patch the target entry to jump to a **local stub entry** (makeFuncStubEntry) so the patch stays short.
// - Use PtrCodeWithCtxTrampoline and call original via FixOrigin trampoline funcval (no Unpatch/Restore inside hook).
// - Always patch ABIInternal reflect.callReflect (see ensureCallReflectPatched in dumpfunc_go124_arm64.go).

var (
	DebugLastProxyFuncValPtr uintptr
	DebugLastProxyCodePtr    uintptr
	DebugLastMakeFuncStubPtr uintptr
	DebugOrigFuncValPtr      uintptr
	DebugOrigCodePtr         uintptr
	guardKeepAlive           sync.Map // map[*patch.Guard]any
)

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
	origFuncVal := bytecode.GetPtr(v)
	DebugOrigFuncValPtr = uintptr(origFuncVal)
	DebugOrigCodePtr = *(*uintptr)(origFuncVal)
	tracePatchFuncPtr("PatchFunc", originPtr, origFuncVal, t)
	return PatchFuncPtr(originPtr, origFuncVal, t)
}

func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	if originPtr == 0 {
		return nil, errors.New("argdump: PatchFuncPtr originPtr=0")
	}
	if typ == nil || typ.Kind() != reflect.Func {
		return nil, errors.New("argdump: PatchFuncPtr typ must be a func type")
	}
	tracePatchFuncPtr("PatchFuncPtr", originPtr, origFuncVal, typ)

	ensureCallReflectPatched()

	makeFuncStubPtr, err := unexports2.FindFuncByName("reflect.makeFuncStub")
	if err != nil {
		return nil, err
	}
	makeFuncStubEntryPtr := reflect.ValueOf(makeFuncStubEntry).Pointer()

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
	// Size metadata required by runtime.reflectcall on go1.18+.
	impl.stackArgsSz = uint32(abid.frameTypeSizeBytes())
	impl.stackRetOff = uint32(abid.retOffset)
	impl.frameSize = uint32(abid.frameSizeBytes())

	ctxtPtr := uintptr(unsafe.Pointer(impl))
	DebugLastProxyFuncValPtr = ctxtPtr
	DebugLastProxyCodePtr = makeFuncStubEntryPtr
	DebugLastMakeFuncStubPtr = makeFuncStubPtr

	tracePatchFuncPtr("PatchFuncPtr.beforePatch", originPtr, origFuncVal, typ)
	guard, err := patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubEntryPtr)
	if err != nil {
		tracePatchFuncErr("PatchFuncPtr.patchErr", err)
		// If the wrapper entry contains an internal branch target within the first patch window,
		// `internal/patch` refuses to patch it. In that case, retry by patching the ABIInternal
		// implementation (the wrapper's inner target) instead.
		//
		// This is common for some wrappers generated around abi0/ABIInternal boundaries.
		if strings.Contains(err.Error(), "branch jumps into the first") {
			if inner, e := bytecode.GetInnerFunc(64, originPtr); e == nil && inner != 0 && inner != originPtr {
				tracePtr("PatchFuncPtr.inner", inner)
				if fn := runtime.FuncForPC(inner); fn != nil && strings.HasPrefix(fn.Name(), "runtime.") {
					return nil, fmt.Errorf("argdump: inner target is runtime (%s), refusing fallback", fn.Name())
				}
				originPtr = inner
				tracePatchFuncPtr("PatchFuncPtr.afterInner", originPtr, origFuncVal, typ)
				guard, err = patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubEntryPtr)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	impl.hookGuard = guard
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr guard fixOrigin=0x%x\n", guard.FixOriginFunc())
	}
	if guard.FixOriginFunc() == 0 {
		return nil, errors.New("argdump: PatchFunc trampoline not available (FixOriginFunc=0)")
	}

	// Build a trampoline funcval with the original ctxt (if any). This allows closures to work
	// without unpatching: runtime.reflectcall will use ctxt on the call path.
	type funcval struct {
		fn   uintptr
		ctxt uintptr
	}
	origCtxt := uintptr(0)
	if origFuncVal != nil {
		origCtxt = *(*uintptr)(unsafe.Add(origFuncVal, ptrSize))
	}
	fv := &funcval{fn: guard.FixOriginFunc(), ctxt: origCtxt}
	impl.origFuncValBox = fv
	impl.origFuncVal = unsafe.Pointer(fv)
	// IMPORTANT: call the original via trampoline (no Unpatch/Restore).
	impl.useTrampoline = true

	guardKeepAlive.Store(guard, impl)
	runtime.SetFinalizer(guard, func(g *patch.Guard) {
		guardKeepAlive.Delete(g)
	})
	return guard, nil
}

func tracePatchFuncPtr(label string, originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) {
	if os.Getenv("GOOM_TRACE_MPROTECT") != "1" {
		return
	}
	originName := "<unknown>"
	if fn := runtime.FuncForPC(originPtr); fn != nil {
		originName = fn.Name()
	}
	codePtr := uintptr(0)
	if origFuncVal != nil {
		codePtr = *(*uintptr)(origFuncVal)
	}
	codeName := "<unknown>"
	if codePtr != 0 {
		if fn := runtime.FuncForPC(codePtr); fn != nil {
			codeName = fn.Name()
		}
	}
	typeName := "<unknown>"
	if typ != nil {
		typeName = typ.String()
	}
	_, _ = fmt.Fprintf(os.Stderr, "[argdump] %s origin=0x%x originFunc=%s type=%s funcval=0x%x codePtr=0x%x codeFunc=%s\n",
		label, originPtr, originName, typeName, uintptr(origFuncVal), codePtr, codeName)
}

func tracePatchFuncErr(label string, err error) {
	if os.Getenv("GOOM_TRACE_MPROTECT") != "1" {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[argdump] %s err=%v\n", label, err)
}

func tracePtr(label string, ptr uintptr) {
	if os.Getenv("GOOM_TRACE_MPROTECT") != "1" {
		return
	}
	name := "<unknown>"
	if fn := runtime.FuncForPC(ptr); fn != nil {
		name = fn.Name()
	}
	_, _ = fmt.Fprintf(os.Stderr, "[argdump] %s ptr=0x%x func=%s\n", label, ptr, name)
}
