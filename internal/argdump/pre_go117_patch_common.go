//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

var guardKeepAlive sync.Map // map[*patch.Guard]any

// PatchFunc patches the provided function so that every call will:
// - print all arguments using EncodeJSONFromAddr
// - then continue executing the original function
// - and preserve the original return values
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	if fn == nil {
		return nil, errors.New("argdump: PatchFunc nil")
	}
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return nil, errors.New("argdump: PatchFunc expects func")
	}
	v := reflect.ValueOf(fn)
	originPtr := v.Pointer()
	origFuncVal := bytecode.GetPtr(v)
	return PatchFuncPtr(originPtr, origFuncVal, t)
}

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
			fn:     makeFuncStubPtr,
			stack:  abid.stackPtrs,
			argLen: abid.stackCallArgsSize,
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	impl.origFuncVal = origFuncVal
	impl.stackArgsSz = uint32(abid.stackCallArgsSize)
	impl.stackRetOff = uint32(abid.retOffset)

	fnName := ""
	if f := runtime.FuncForPC(originPtr); f != nil {
		fnName = f.Name()
	}
	impl.useTrampoline = fnName != "" && !strings.Contains(fnName, ".func")

	ctxtPtr := uintptr(unsafe.Pointer(impl))

	guard, err := patchPreGo117Entry(originPtr, ctxtPtr, makeFuncStubPtr, impl)
	if err != nil {
		return nil, err
	}

	impl.hookGuard = guard
	guardKeepAlive.Store(guard, impl)
	runtime.SetFinalizer(guard, func(g *patch.Guard) {
		guardKeepAlive.Delete(g)
	})
	return guard, nil
}
