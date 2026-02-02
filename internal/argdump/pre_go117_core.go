//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/abijson"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// Shared core for go1.13-go1.16 across architectures (ABI0 stack-only).
//
// Key properties:
// - Reuse stdlib reflect.makeFuncStub (keeps runtime stack map invariants).
// - Patch reflect.callReflect -> callDump to avoid reflect.Value.
// - Read args/returns directly from the arg frame (stack-only ABI).

const ptrSize = uintptr(unsafe.Sizeof(uintptr(0)))

// NOTE: must agree with runtime.bitvector expectations for go1.13-go1.16.
// (Matches go1.16 reflect.bitVector.)
type bitVector struct {
	n    uint32
	data []byte
}

func (bv *bitVector) append(bit uint8) {
	if bv.n%8 == 0 {
		bv.data = append(bv.data, 0)
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}

// makeFuncCtxt matches the prefix of go1.13-go1.16 reflect.makeFuncImpl.
type makeFuncCtxt struct {
	fn     uintptr
	stack  *bitVector // ptrmap for args+results
	argLen uintptr    // args only
}

type dumpFuncImpl struct {
	makeFuncCtxt

	magic uintptr

	fnType reflect.Type
	abid   abiDesc

	hookGuard      *patch.Guard
	origFuncVal    unsafe.Pointer
	origFuncValBox interface{}
	useTrampoline  bool // only meaningful on arches where patch supports it

	stackArgsSz uint32
	stackRetOff uint32
}

var (
	patchOnce sync.Once

	origCallReflect func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool)

	magicSentinel = new(int)
)

//go:noinline
func callReflectTrampolineHolder(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool) {
	var x uintptr
	x ^= uintptr(ctxt)
	x ^= uintptr(frame)
	if retValid != nil && *retValid {
		x++
	}
	for i := 0; i < 64; i++ {
		// Keep constants within 32-bit uintptr range (also fine on 64-bit).
		x ^= uintptr(i) * uintptr(0x9e3779b9)
	}
	if x == 0xdeadbeef {
		panic("unreachable")
	}
}

func ensureCallReflectPatched() {
	patchOnce.Do(func() {
		callReflectPtr, err := unexports2.FindFuncByName("reflect.callReflect")
		if err != nil {
			panic(err)
		}

		guard, err := patch.PtrTrampoline(callReflectPtr, callDump, callReflectTrampolineHolder)
		if err != nil {
			panic(err)
		}

		var tmp func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool)
		_, err = unexports2.CreateFuncForCodePtr(&tmp, guard.FixOriginFunc())
		if err != nil {
			panic(err)
		}
		origCallReflect = tmp

		guard.Apply()
	})
}

// MakeDumpFunc returns a function value of the provided function type.
// The returned interface{} has dynamic type == typ.
func MakeDumpFunc(typ reflect.Type) interface{} {
	if typ == nil || typ.Kind() != reflect.Func {
		panic("argdump: typ must be a non-nil func type")
	}
	ensureCallReflectPatched()

	makeFuncStubPtr, err := unexports2.FindFuncByName("reflect.makeFuncStub")
	if err != nil {
		panic(err)
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

	return packEface(rtypePtr(typ), unsafe.Pointer(impl))
}

// callDump is installed in place of reflect.callReflect.
func callDump(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool) {
	if ctxt != nil {
		impl := (*dumpFuncImpl)(ctxt)
		if impl.magic == uintptr(unsafe.Pointer(magicSentinel)) && impl.fnType != nil {
			dumpArgsAndMaybeCall(impl, frame, retValid)
			return
		}
	}
	if origCallReflect == nil {
		panic("argdump: original callReflect trampoline not initialized")
	}
	origCallReflect(ctxt, frame, retValid)
}

func dumpArgsAndMaybeCall(impl *dumpFuncImpl, frame unsafe.Pointer, retValid *bool) {
	opt := abijson.DefaultOptions()
	opt.MaxDepth = 3

	if DumpEnabled {
		for i := 0; i < impl.fnType.NumIn(); i++ {
			at := impl.fnType.In(i)
			if at.Size() == 0 {
				fmt.Printf("arg%d=%s\n", i, "null")
				continue
			}
			addr := impl.abid.addrOfArg(i, frame)
			if addr == nil {
				fmt.Printf("arg%d=%s\n", i, `"<unavailable>"`)
				continue
			}
			b, err := abijson.EncodeJSONFromAddrWithOptions(at, addr, opt)
			if err != nil {
				fmt.Printf("arg%d=%q\n", i, err.Error())
				continue
			}
			fmt.Printf("arg%d=%s\n", i, string(b))
		}
	}

	if impl.hookGuard != nil && impl.origFuncVal != nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				if DumpEnabled {
					dumpPanic(recovered)
				}
				panic(recovered)
			}
		}()

		callOriginalPreGo117(impl, frame)
		if retValid != nil {
			*retValid = true
		}

		if DumpEnabled {
			for i := 0; i < impl.fnType.NumOut(); i++ {
				rt := impl.fnType.Out(i)
				if rt.Size() == 0 {
					fmt.Printf("ret%d=%s\n", i, "null")
					continue
				}
				addr := impl.abid.addrOfRet(i, frame)
				if addr == nil {
					fmt.Printf("ret%d=%s\n", i, `"<unavailable>"`)
					continue
				}
				b, err := abijson.EncodeJSONFromAddrWithOptions(rt, addr, opt)
				if err != nil {
					fmt.Printf("ret%d=%q\n", i, err.Error())
					continue
				}
				fmt.Printf("ret%d=%s\n", i, string(b))
			}
		}
		return
	}

	impl.abid.zeroRets(impl.fnType, frame)
	if retValid != nil {
		*retValid = true
	}
}

func dumpPanic(v interface{}) {
	if v == nil {
		fmt.Printf("panic=%s\n", "null")
		return
	}
	var s string
	switch x := v.(type) {
	case string:
		s = x
	case error:
		s = x.Error()
	case interface{ String() string }:
		s = x.String()
	default:
		s = fmt.Sprintf("%T: %v", v, v)
	}
	b, _ := json.Marshal(s)
	fmt.Printf("panic=%s\n", string(b))
}

//go:linkname runtimeReflectcall runtime.reflectcall
func runtimeReflectcall(argtype unsafe.Pointer, fn, arg unsafe.Pointer, argsize uint32, retoffset uint32)

func rtypePtr(t reflect.Type) unsafe.Pointer {
	type iface struct {
		tab  unsafe.Pointer
		data unsafe.Pointer
	}
	return (*iface)(unsafe.Pointer(&t)).data
}

type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

func packEface(typ, data unsafe.Pointer) interface{} {
	e := eface{typ: typ, data: data}
	return *(*interface{})(unsafe.Pointer(&e))
}

type abiDesc struct {
	stackCallArgsSize uintptr
	retOffset         uintptr
	stackPtrs         *bitVector

	inOffs  []uintptr
	outOffs []uintptr
}

func align(x, a uintptr) uintptr { return (x + a - 1) &^ (a - 1) }

func newAbiDescFromFuncType(t reflect.Type) abiDesc {
	ptrmap := new(bitVector)
	offset := uintptr(0)

	inOffs := make([]uintptr, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		arg := t.In(i)
		offset = align(offset, uintptr(arg.Align()))
		inOffs[i] = offset
		addTypeBits(ptrmap, offset, arg)
		offset += arg.Size()
	}
	stackCallArgsSize := offset

	offset = align(offset, ptrSize)
	retOffset := offset

	outOffs := make([]uintptr, t.NumOut())
	for i := 0; i < t.NumOut(); i++ {
		res := t.Out(i)
		offset = align(offset, uintptr(res.Align()))
		outOffs[i] = offset
		addTypeBits(ptrmap, offset, res)
		offset += res.Size()
	}
	_ = align(offset, ptrSize)

	return abiDesc{
		stackCallArgsSize: stackCallArgsSize,
		retOffset:         retOffset,
		stackPtrs:         ptrmap,
		inOffs:            inOffs,
		outOffs:           outOffs,
	}
}

func (a abiDesc) addrOfArg(i int, frame unsafe.Pointer) unsafe.Pointer {
	if i < 0 || i >= len(a.inOffs) {
		return nil
	}
	return unsafe.Pointer(uintptr(frame) + a.inOffs[i])
}

func (a abiDesc) addrOfRet(i int, frame unsafe.Pointer) unsafe.Pointer {
	if i < 0 || i >= len(a.outOffs) {
		return nil
	}
	return unsafe.Pointer(uintptr(frame) + a.outOffs[i])
}

func (a abiDesc) zeroRets(fnType reflect.Type, frame unsafe.Pointer) {
	for i := 0; i < fnType.NumOut(); i++ {
		rt := fnType.Out(i)
		if rt.Size() == 0 {
			continue
		}
		memclr(unsafe.Pointer(uintptr(frame)+a.outOffs[i]), rt.Size())
	}
}

func typeHasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice, reflect.String, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return typeHasPointers(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if typeHasPointers(t.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func addTypeBits(bv *bitVector, offset uintptr, t reflect.Type) {
	if !typeHasPointers(t) {
		return
	}
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice, reflect.String, reflect.UnsafePointer:
		for bv.n < uint32(offset/ptrSize) {
			bv.append(0)
		}
		bv.append(1)
	case reflect.Interface:
		for bv.n < uint32(offset/ptrSize) {
			bv.append(0)
		}
		bv.append(1)
		bv.append(1)
	case reflect.Array:
		elem := t.Elem()
		for i := 0; i < t.Len(); i++ {
			addTypeBits(bv, offset+uintptr(i)*elem.Size(), elem)
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			addTypeBits(bv, offset+f.Offset, f.Type)
		}
	}
}

func memclr(p unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(uintptr(p) + i)) = 0
	}
}
