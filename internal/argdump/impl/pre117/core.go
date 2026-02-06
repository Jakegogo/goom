//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package pre117

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/abijson"
	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/internal/bitvec"
	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/argdump/internal/mem"
	"github.com/tencent/goom/internal/argdump/internal/util"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// Shared core for go1.13-go1.16 across architectures (ABI0 stack-only).
//
// Key properties:
// - Reuse stdlib reflect.makeFuncStub (keeps runtime stack map invariants).
// - Patch reflect.callReflect -> callDump to avoid reflect.Value.
// - Read args/returns directly from the arg frame (stack-only ABI).

// makeFuncCtxt matches the prefix of go1.13-go1.16 reflect.makeFuncImpl.
type makeFuncCtxt struct {
	fn     uintptr
	stack  *bitvec.BitVector // ptrmap for args+results
	argLen uintptr           // args only
}

// DumpFuncImpl is the implementation of a dump function.
type DumpFuncImpl struct {
	makeFuncCtxt

	magic uintptr

	fnType reflect.Type
	abid   abiDesc

	HookGuard      *patch.Guard
	OrigFuncVal    unsafe.Pointer
	OrigFuncValBox interface{}
	UseTrampoline  bool // only meaningful on arches where patch supports it

	StackArgsSz uint32
	StackRetOff uint32
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
	impl := &DumpFuncImpl{
		makeFuncCtxt: makeFuncCtxt{
			fn:     makeFuncStubPtr,
			stack:  abid.stackPtrs,
			argLen: abid.stackCallArgsSize,
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	return util.PackEface(util.RtypePtr(typ), unsafe.Pointer(impl))
}

// callDump is installed in place of reflect.callReflect.
func callDump(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool) {
	if ctxt != nil {
		impl := (*DumpFuncImpl)(ctxt)
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

func dumpArgsAndMaybeCall(impl *DumpFuncImpl, frame unsafe.Pointer, retValid *bool) {
	opt := abijson.DefaultOptions()
	opt.MaxDepth = 3

	if flags.DumpEnabled {
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

	if impl.HookGuard != nil && impl.OrigFuncVal != nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				if flags.DumpEnabled {
					dumpPanic(recovered)
				}
				panic(recovered)
			}
		}()

		CallOriginal(impl, frame)
		if retValid != nil {
			*retValid = true
		}

		if flags.DumpEnabled {
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

type abiDesc struct {
	stackCallArgsSize uintptr
	retOffset         uintptr
	stackPtrs         *bitvec.BitVector

	inOffs  []uintptr
	outOffs []uintptr
}

func newAbiDescFromFuncType(t reflect.Type) abiDesc {
	ptrmap := new(bitvec.BitVector)
	offset := uintptr(0)

	inOffs := make([]uintptr, t.NumIn())
	for i := 0; i < t.NumIn(); i++ {
		arg := t.In(i)
		offset = util.Align(offset, uintptr(arg.Align()))
		inOffs[i] = offset
		util.AddTypeBits(ptrmap, offset, arg)
		offset += arg.Size()
	}
	stackCallArgsSize := offset

	offset = util.Align(offset, abi.PtrSize)
	retOffset := offset

	outOffs := make([]uintptr, t.NumOut())
	for i := 0; i < t.NumOut(); i++ {
		res := t.Out(i)
		offset = util.Align(offset, uintptr(res.Align()))
		outOffs[i] = offset
		util.AddTypeBits(ptrmap, offset, res)
		offset += res.Size()
	}
	_ = util.Align(offset, abi.PtrSize)

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
		mem.Memclr(unsafe.Pointer(uintptr(frame)+a.outOffs[i]), rt.Size())
	}
}
