//go:build go1.17 && !go1.18 && arm64
// +build go1.17,!go1.18,arm64

package go117

import (
	"encoding/json"
	"fmt"
	"os"
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

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive for go1.17+).
type makeFuncCtxt struct {
	fn      uintptr
	stack   *bitvec.BitVector
	argLen  uintptr
	regPtrs intArgRegBitmap
}

// DumpFuncImpl is the dump function implementation.
type DumpFuncImpl struct {
	makeFuncCtxt

	magic uintptr

	fnType reflect.Type
	abid   abiDesc

	HookGuard      *patch.Guard
	OrigFn         interface{}
	OrigFuncVal    unsafe.Pointer
	OrigFuncValBox interface{}
	UseTrampoline  bool
	StackArgsSz    uint32
	StackRetOff    uint32
	FrameSize      uint32
}

var (
	patchOnce sync.Once

	origCallReflect func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)

	magicSentinel = new(int)
)

//go:noinline
func callReflectTrampolineHolder(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	var x uintptr
	x ^= uintptr(ctxt)
	x ^= uintptr(frame)
	if retValid != nil && *retValid {
		x++
	}
	if regs != nil {
		x ^= uintptr(regs)
	}
	for i := 0; i < 64; i++ {
		x ^= uintptr(i) * 0x9e3779b97f4a7c15
	}
	if x == 0xdeadbeef {
		panic("unreachable")
	}
}

func ensureCallReflectPatched() {
	patchOnce.Do(func() {
		callReflectPtr, err := unexports2.FindFuncByName("reflect.callReflect")
		if err != nil || callReflectPtr == 0 {
			panic("argdump: failed to resolve reflect.callReflect")
		}

		flags.DebugCallReflectPtr = callReflectPtr
		flags.DebugCallDumpPtr = reflect.ValueOf(callDump).Pointer()

		guard, err := patch.PtrTrampoline(callReflectPtr, callDump, callReflectTrampolineHolder)
		if err != nil {
			panic(err)
		}

		var tmp func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)
		_, err = unexports2.CreateFuncForCodePtr(&tmp, guard.FixOriginFunc())
		if err != nil {
			panic(err)
		}
		origCallReflect = tmp

		guard.Apply()
	})
}

// MakeDumpFunc returns a function value of the provided function type.
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
			fn:      makeFuncStubPtr,
			stack:   abid.stackPtrs,
			argLen:  abid.stackCallArgsSize,
			regPtrs: intArgRegBitmap{},
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	return util.PackEface(util.RtypePtr(typ), unsafe.Pointer(impl))
}

func callDump(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	if flags.DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] callDump entry ctxt=%p frame=%p regs=%p\n", ctxt, frame, regs)
	}
	if ctxt != nil {
		impl := (*DumpFuncImpl)(ctxt)
		if impl.magic == uintptr(unsafe.Pointer(magicSentinel)) && impl.fnType != nil {
			dumpArgsAndZeroRets(impl, frame, retValid)
			return
		}
	}
	if origCallReflect == nil {
		panic("argdump: original callReflect trampoline not initialized")
	}
	origCallReflect(ctxt, frame, retValid, regs)
}

func dumpArgsAndZeroRets(impl *DumpFuncImpl, frame unsafe.Pointer, retValid *bool) {
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

	if impl.HookGuard != nil && impl.OrigFn != nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				if flags.DumpEnabled {
					dumpPanic(recovered)
				}
				panic(recovered)
			}
		}()

		in := make([]reflect.Value, impl.fnType.NumIn())
		for i := 0; i < impl.fnType.NumIn(); i++ {
			at := impl.fnType.In(i)
			if at.Size() == 0 {
				in[i] = reflect.Zero(at)
				continue
			}
			addr := impl.abid.addrOfArg(i, frame)
			in[i] = reflect.NewAt(at, addr).Elem()
		}

		orig := reflect.ValueOf(impl.OrigFn)
		var out []reflect.Value
		if impl.fnType.IsVariadic() {
			out = orig.CallSlice(in)
		} else {
			out = orig.Call(in)
		}

		for i := 0; i < impl.fnType.NumOut(); i++ {
			rt := impl.fnType.Out(i)
			if rt.Size() == 0 {
				continue
			}
			addr := impl.abid.addrOfRet(i, frame)
			if addr == nil {
				continue
			}
			reflect.NewAt(rt, addr).Elem().Set(out[i])
		}
		if retValid != nil {
			*retValid = true
		}
	} else {
		impl.abid.zeroRets(impl.fnType, frame)
		if retValid != nil {
			*retValid = true
		}
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

type intArgRegBitmap [(abi.IntArgRegs + 7) / 8]uint8

func (b *intArgRegBitmap) Set(i int) { b[i/8] |= uint8(1) << (i % 8) }
func (b *intArgRegBitmap) Get(i int) bool {
	return b[i/8]&(uint8(1)<<(i%8)) != 0
}
