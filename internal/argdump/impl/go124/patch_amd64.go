//go:build go1.24 && amd64
// +build go1.24,amd64

package go124

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

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

	impl.OrigFuncVal = origFuncVal
	impl.StackArgsSz = uint32(abid.frameTypeSizeBytes())
	impl.StackRetOff = uint32(abid.retOffset)
	impl.FrameSize = uint32(abid.frameSizeBytes())
	if flags.DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr impl=%p fn=0x%x firstWord=0x%x\n",
			impl, impl.makeFuncCtxt.fn, *(*uintptr)(unsafe.Pointer(impl)),
		)
	}

	ctxtPtr := uintptr(unsafe.Pointer(impl))
	flags.DebugLastProxyFuncValPtr = ctxtPtr
	flags.DebugLastProxyCodePtr = makeFuncStubPtr
	flags.DebugLastMakeFuncStubPtr = makeFuncStubPtr

	fnName := ""
	if f := runtime.FuncForPC(originPtr); f != nil {
		fnName = f.Name()
	}
	impl.UseTrampoline = fnName != "" && !strings.Contains(fnName, ".func")

	var guard *patch.Guard
	if impl.UseTrampoline {
		guard, err = patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubPtr)
		if err == nil && guard.FixOriginFunc() != 0 {
			type funcval struct {
				fn   uintptr
				ctxt uintptr
			}
			fv := &funcval{fn: guard.FixOriginFunc()}
			impl.OrigFuncValBox = fv
			impl.OrigFuncVal = unsafe.Pointer(fv)
		} else {
			impl.UseTrampoline = false
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
	impl.HookGuard = guard
	guardKeepAlive.Store(guard, impl)
	runtime.SetFinalizer(guard, func(g *patch.Guard) {
		guardKeepAlive.Delete(g)
	})
	return guard, nil
}

