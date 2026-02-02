//go:build go1.17 && !go1.18 && arm64
// +build go1.17,!go1.18,arm64

package argdump

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/abijson"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// go1.17 (arm64):
//
// Key stability note:
// - In go1.17 arm64, reflect.makeFuncStub passes regs=nil into reflect.callReflect (see reflect/asm_arm64.s),
//   so the MakeFunc call path is effectively stack-only.
// - In go1.18+ arm64, makeFuncStub starts passing a non-nil regs pointer (abi.RegArgs) and spills reg args.
//   Treating go1.18 like go1.17 caused SIGBUS/segfault in earlier iterations (reading pointers from wrong place).
//
// Therefore this implementation:
// - patches the ABIInternal implementation of reflect.callReflect -> callDump
// - dumps args/returns using stack-only offsets from the arg frame
// - zeroes return values in the stack result area
//
// PatchFunc for arbitrary targets is intentionally disabled pre-go1.24 (see patchfunc_go117_to_go123_stub.go).

// bitVector must agree with reflect.bitVector for go1.17-go1.23.
// Runtime reads {n uint32; bytedata *uint8} where bytedata points at the slice data.
type bitVector struct {
	n    uint32
	data []byte
}

func (bv *bitVector) append(bit uint8) {
	// Keep pointer masks aligned to uintptr boundaries (safe across versions).
	if bv.n%(8*uint32(ptrSize)) == 0 {
		for i := uintptr(0); i < ptrSize; i++ {
			bv.data = append(bv.data, 0)
		}
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive for go1.17+).
type makeFuncCtxt struct {
	fn      uintptr
	stack   *bitVector
	argLen  uintptr
	regPtrs intArgRegBitmap
}

type dumpFuncImpl struct {
	makeFuncCtxt

	magic uintptr

	fnType reflect.Type
	abid   abiDesc

	// Present for API compatibility with go1.24 impl; unused in this pre-go1.24 build.
	hookGuard      *patch.Guard
	origFn         interface{} // typed func value reconstructed from *runtime.FuncVal
	origFuncVal    unsafe.Pointer
	origFuncValBox interface{}
	useTrampoline  bool
	stackArgsSz    uint32
	stackRetOff    uint32
	frameSize      uint32
}

var (
	patchOnce sync.Once

	origCallReflect func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)

	magicSentinel = new(int)
)

// DebugCallDumpPtr is the entry PC for callDump (diagnostics only).
var DebugCallDumpPtr uintptr

// DebugCallReflectPtr is the resolved entry PC for reflect.callReflect (diagnostics only).
var DebugCallReflectPtr uintptr

// DebugEnabled enables verbose diagnostics from callDump (stderr).
var DebugEnabled bool

// DumpEnabled controls whether argdump prints/encodes arguments/returns/panics.
var DumpEnabled = true

//go:noinline
func callReflectTrampolineHolder(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	// Keep this function "large" so the trampoline copier has room.
	// This code will never execute once patched.
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
		// go1.17-go1.23 MakeFunc stubs on arm64 pass regs=nil and use the stack arg frame.
		// Patch the public reflect.callReflect symbol directly (stable across these versions).
		callReflectPtr, err := unexports2.FindFuncByName("reflect.callReflect")
		if err != nil || callReflectPtr == 0 {
			panic("argdump: failed to resolve reflect.callReflect")
		}

		DebugCallReflectPtr = callReflectPtr
		DebugCallDumpPtr = reflect.ValueOf(callDump).Pointer()

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
// The returned interface{} has dynamic type == typ (so it can be type-asserted).
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
			// regPtrs is unused on this path because makeFuncStub passes regs=nil.
			regPtrs: intArgRegBitmap{},
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	return packEface(rtypePtr(typ), unsafe.Pointer(impl))
}

// callDump is installed in place of reflect.callReflect.
// regs is expected to be nil on go1.17-go1.23 arm64 MakeFunc stubs.
func callDump(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] callDump entry ctxt=%p frame=%p regs=%p\n", ctxt, frame, regs)
	}
	if ctxt != nil {
		impl := (*dumpFuncImpl)(ctxt)
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

func dumpArgsAndZeroRets(impl *dumpFuncImpl, frame unsafe.Pointer, retValid *bool) {
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
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] after dump args frame=%p\n", frame)
	}

	// If PatchFunc installed a hook guard, call the original and preserve returns.
	//
	// Deployment & stability notes (go1.17/darwin/arm64):
	// - Apple Silicon enforces W^X. Writing to .text typically requires temporarily removing EXEC from a whole page.
	// - If we Unpatch/Restore the origin inside this hook, we may toggle permissions on a page that contains
	//   currently executing code (including parts of the test binary), causing an immediate SIGBUS/SIGSEGV.
	// - Therefore, for go1.17 we call the original via Guard.FixOriginFunc trampoline (impl.origFn) and keep the
	//   origin patch applied for the entire duration of the call.
	if impl.hookGuard != nil && impl.origFn != nil {
		defer func() {
			if recovered := recover(); recovered != nil {
				if DumpEnabled {
					dumpPanic(recovered)
				}
				panic(recovered)
			}
		}()
		if DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] calling original: building reflect args")
		}

		// Build []reflect.Value args from the stack arg frame.
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

		if DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] calling original: reflect.Call")
		}
		orig := reflect.ValueOf(impl.origFn)
		var out []reflect.Value
		// reflect.Call for variadic functions expects expanded args (a,b,c...), not the slice.
		// However, at the callee boundary the variadic args are already materialized as a slice
		// in our frame (last parameter type is []T). So we must use CallSlice here.
		if impl.fnType.IsVariadic() {
			out = orig.CallSlice(in)
		} else {
			out = orig.Call(in)
		}
		if DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] calling original: reflect.Call returned; writing returns")
		}

		// Materialize returns back into the stack return area for the caller.
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
		// Default (MakeDumpFunc): zero returns.
		impl.abid.zeroRets(impl.fnType, frame)
		if retValid != nil {
			*retValid = true
		}
	}

	if DumpEnabled {
		for i := 0; i < impl.fnType.NumOut(); i++ {
			rt := impl.fnType.Out(i)
			if rt.Size() == 0 {
				fmt.Printf("ret%d=%s\n", i, "null")
				continue
			}
			addr := impl.abid.addrOfRet(i, frame)
			if DebugEnabled && addr != nil && rt.Kind() == reflect.Int {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] ret%d addr=%p int=%d\n", i, addr, *(*int)(addr))
			}
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
	if DebugEnabled {
		_, _ = fmt.Fprintln(os.Stderr, "[argdump] done (returns written/dumped); returning to makeFuncStub")
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

// --- ABI0 (stack-only) description for go1.17-go1.23 MakeFunc call path ---

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

// --- helpers (duplicated to avoid pulling in go1.24-only implementation) ---

// intArgRegBitmap matches internal/abi.IntArgRegBitmap.
type intArgRegBitmap [(intArgRegs + 7) / 8]uint8

func (b *intArgRegBitmap) Set(i int) { b[i/8] |= uint8(1) << (i % 8) }
func (b *intArgRegBitmap) Get(i int) bool {
	return b[i/8]&(uint8(1)<<(i%8)) != 0
}

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

func typeHasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, kindPointer, reflect.Slice, reflect.String,
		reflect.Interface, reflect.UnsafePointer:
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
	case reflect.Chan, reflect.Func, reflect.Map, kindPointer, reflect.Slice, reflect.String, reflect.UnsafePointer:
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
		for i := 0; i < t.Len(); i++ {
			addTypeBits(bv, offset+uintptr(i)*t.Elem().Size(), t.Elem())
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			addTypeBits(bv, offset+uintptr(f.Offset), f.Type)
		}
	}
}

func memclr(p unsafe.Pointer, n uintptr) {
	b := unsafe.Slice((*byte)(p), n)
	for i := range b {
		b[i] = 0
	}
}
