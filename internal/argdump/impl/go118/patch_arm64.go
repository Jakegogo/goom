//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

package go118

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

func makeFuncStubEntry()

var guardKeepAlive sync.Map // map[*patch.Guard]any

// PatchFunc patches the provided function so that every call will print all arguments.
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
	flags.DebugOrigFuncValPtr = uintptr(origFuncVal)
	flags.DebugOrigCodePtr = *(*uintptr)(origFuncVal)
	tracePatchFuncPtr("PatchFunc", originPtr, origFuncVal, t)
	return PatchFuncPtr(originPtr, origFuncVal, t)
}

// PatchFuncPtr is PatchFunc but takes an explicit function entry address and function type.
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
	impl := &DumpFuncImpl{
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
	impl.StackArgsSz = uint32(abid.frameTypeSizeBytes())
	impl.StackRetOff = uint32(abid.retOffset)
	impl.FrameSize = uint32(abid.frameSizeBytes())

	ctxtPtr := uintptr(unsafe.Pointer(impl))
	flags.DebugLastProxyFuncValPtr = ctxtPtr
	flags.DebugLastProxyCodePtr = makeFuncStubEntryPtr
	flags.DebugLastMakeFuncStubPtr = makeFuncStubPtr

	tracePatchFuncPtr("PatchFuncPtr.beforePatch", originPtr, origFuncVal, typ)
	guard, err := patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubEntryPtr)
	if err != nil {
		tracePatchFuncErr("PatchFuncPtr.patchErr", err)
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
	impl.HookGuard = guard
	if flags.DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr guard fixOrigin=0x%x\n", guard.FixOriginFunc())
	}
	if guard.FixOriginFunc() == 0 {
		return nil, errors.New("argdump: PatchFunc trampoline not available (FixOriginFunc=0)")
	}

	type funcval struct {
		fn   uintptr
		ctxt uintptr
	}
	origCtxt := uintptr(0)
	if origFuncVal != nil {
		origCtxt = *(*uintptr)(unsafe.Add(origFuncVal, abi.PtrSize))
	}
	fv := &funcval{fn: guard.FixOriginFunc(), ctxt: origCtxt}
	impl.OrigFuncValBox = fv
	impl.OrigFuncVal = unsafe.Pointer(fv)
	impl.UseTrampoline = true

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
