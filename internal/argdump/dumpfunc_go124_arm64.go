//go:build go1.18 && arm64
// +build go1.18,arm64

package argdump

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/abijson"
	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	_ "github.com/tencent/goom/internal/bytecode/stub" // ensure stub symbols exist for //go:linkname users (icache).
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// This package provides a MakeFunc-like facility that produces a function value
// of an arbitrary function type which, when called, prints all arguments by
// calling abijson.EncodeJSONFromAddr on their addresses.
//
// Important: It does not use reflect.Value to access arguments.
//
// Implementation strategy (go1.18+arm64; originally validated on go1.24+arm64):
// - Reuse stdlib reflect.makeFuncStub so runtime stack maps work (see runtime/stkframe.go special-case).
// - Patch reflect.callReflect to our hook. The hook recognizes our generated closures and dumps args.
//   For any other closure, it delegates to the original reflect.callReflect via trampoline.
//
// Key stability note:
// - On go1.18+ arm64, reflect.makeFuncStub spills reg args into an abi.RegArgs area and passes a regs pointer
//   into callReflect/callDump. Our hook must match that ABI and must not corrupt the funcval context register (x26),
//   otherwise reflect.callReflect can crash deep in reflect.funcLayout / runtime type metadata.

// intArgRegBitmap matches internal/abi.IntArgRegBitmap.
type intArgRegBitmap [(intArgRegs + 7) / 8]uint8

func (b *intArgRegBitmap) Set(i int) { b[i/8] |= uint8(1) << (i % 8) }
func (b *intArgRegBitmap) Get(i int) bool {
	return b[i/8]&(uint8(1)<<(i%8)) != 0
}

// regArgs matches internal/abi.RegArgs (layout-sensitive).
type regArgs struct {
	Ints   [intArgRegs]uintptr
	Floats [floatArgRegs]uint64

	Ptrs [intArgRegs]unsafe.Pointer

	ReturnIsPtr intArgRegBitmap
}

func (r *regArgs) intRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
	// All target platforms here are little-endian; no sub-word offset required.
	return unsafe.Pointer(&r.Ints[reg])
}

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive).
type makeFuncCtxt struct {
	fn      uintptr
	stack   *bitVector
	argLen  uintptr
	regPtrs intArgRegBitmap
}

type dumpFuncImpl struct {
	makeFuncCtxt

	// magic distinguishes our closure from reflect.makeFuncImpl / reflect.methodValue.
	magic uintptr

	fnType reflect.Type
	abid   abiDesc

	// Hooking support: if hookGuard != nil, callDump will temporarily unpatch the
	// origin function, call it via runtime.reflectcall, then restore the patch.
	hookGuard      *patch.Guard
	origFuncVal    unsafe.Pointer // *runtime.FuncVal (pointer-authenticated on arm64e)
	origFuncValBox interface{}    // keeps trampoline funcval alive on heap (avoid dangling pointers)
	useTrampoline  bool           // call orig via hookGuard.FixOriginFunc trampoline (no Unpatch/Restore)
	stackArgsSz    uint32
	stackRetOff    uint32
	frameSize      uint32
}

var (
	patchOnce sync.Once

	// trampoline to the original reflect.callReflect, populated after patching.
	origCallReflect func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)
	// callDumpFuncVal keeps the patched target funcval alive (GC does not see text immediates).
	callDumpFuncVal func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)

	// sentinel used for dumpFuncImpl.magic.
	magicSentinel = new(int)
)

// DebugCallDumpPtr is the entry PC for callDump (diagnostics only).
var DebugCallDumpPtr uintptr

// DebugCallReflectPtr is the resolved entry PC for reflect.callReflect (diagnostics only).
var DebugCallReflectPtr uintptr

// DebugRuntimeReflectcallPtr is the entry PC for runtime.reflectcall (diagnostics only).
var DebugRuntimeReflectcallPtr uintptr

// DebugRuntimeSpillArgsPtr/DebugRuntimeUnspillArgsPtr are entry PCs for runtime.{spillArgs,unspillArgs}.
var DebugRuntimeSpillArgsPtr uintptr
var DebugRuntimeUnspillArgsPtr uintptr

// DebugMoveMakeFuncArgPtrsPtr is entry PC for reflect.moveMakeFuncArgPtrs (ABIInternal).
var DebugMoveMakeFuncArgPtrsPtr uintptr

// DebugEnabled enables verbose diagnostics from callDump (stderr).
// Keep this off by default; it is intended for debugging SIGBUS/patching issues.
var DebugEnabled bool

// DumpEnabled controls whether argdump prints/encodes arguments/returns/panics.
// When false, PatchFunc still intercepts and continues execution, but does not do
// JSON encoding nor I/O, allowing us to measure the hook/dispatch overhead.
var DumpEnabled = true

// MakeDumpFunc returns a function value of the provided function type.
// The returned interface{} has dynamic type == typ (so it can be type-asserted).
//
// The generated function prints all arguments as JSON and returns zero values.
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
			fn:      makeFuncStubPtr,
			stack:   abid.stackPtrs,
			argLen:  abid.stackCallArgsSize,
			regPtrs: abid.inRegPtrs,
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	// Build an interface{} value whose dynamic type is typ and whose data word is impl.
	return packEface(rtypePtr(typ), unsafe.Pointer(impl))
}

func ensureCallReflectPatched() {
	patchOnce.Do(func() {
		// Critical: patch the ABIInternal implementation of reflect.callReflect.
		// Prefer decoding an explicit ABI0 wrapper symbol if present; otherwise use the
		// ABIInternal symbol directly. Avoid resolving via makeFuncStub because its
		// first call site is runtime.spillArgs on go1.18-go1.23.
		callReflectPtr := uintptr(0)
		if callReflectAbi0Ptr, e := unexports2.FindFuncByName("reflect.callReflect.abi0"); e == nil && callReflectAbi0Ptr != 0 {
			if inner, e2 := bytecode.GetInnerFunc(0, callReflectAbi0Ptr); e2 == nil && inner != 0 && inner != callReflectAbi0Ptr {
				callReflectPtr = inner
			}
			if DebugEnabled {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] resolve callReflect.abi0=0x%x inner=0x%x\n", callReflectAbi0Ptr, callReflectPtr)
			}
		}
		// Otherwise, patch reflect.callReflect directly.
		if callReflectPtr == 0 {
			p, err := unexports2.FindFuncByName("reflect.callReflect")
			if err != nil {
				panic(err)
			}
			callReflectPtr = p
			if DebugEnabled {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] resolve callReflect=0x%x\n", p)
			}
		}
		DebugCallReflectPtr = callReflectPtr
		DebugCallDumpPtr = reflect.ValueOf(callDump).Pointer()
		DebugRuntimeReflectcallPtr = reflect.ValueOf(runtimeReflectcall).Pointer()

		if p, e := unexports2.FindFuncByName("runtime.spillArgs"); e == nil {
			DebugRuntimeSpillArgsPtr = p
		}
		if p, e := unexports2.FindFuncByName("runtime.unspillArgs"); e == nil {
			DebugRuntimeUnspillArgsPtr = p
		}
		if p, e := unexports2.FindFuncByName("reflect.moveMakeFuncArgPtrs"); e == nil {
			DebugMoveMakeFuncArgPtrsPtr = p
		}

		// Prepare a trampoline before applying the patch.
		// Important: creating origCallReflect via reflect.MakeFunc would recurse back into callReflect,
		// so we must create origCallReflect (pointing at the fixed origin) BEFORE guard.Apply().
		var tmp func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)
		_, err := unexports2.CreateFuncForCodePtr(&tmp, reflect.ValueOf(callDump).Pointer())
		if err != nil {
			panic(err)
		}
		callDumpFuncVal = tmp

		guard, err := patch.PtrTrampoline(callReflectPtr, callDumpFuncVal, callReflectTrampolineHolder)
		if err != nil {
			panic(err)
		}

		// Make origCallReflect callable by wiring a function value to the fixed origin.
		_, err = unexports2.CreateFuncForCodePtr(&tmp, guard.FixOriginFunc())
		if err != nil {
			panic(err)
		}
		origCallReflect = tmp

		guard.Apply()
	})
}

// callReflectTrampolineHolder is a placeholder function whose code will be overwritten
// with the fixed origin instructions by patch.PtrTrampoline.
//
//go:noinline
func callReflectTrampolineHolder(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	// Keep this function "large" so the trampoline copier has room.
	// This code will never execute once patched.
	var x uintptr
	x ^= uintptr(unsafe.Pointer(ctxt))
	x ^= uintptr(unsafe.Pointer(frame))
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

// callDump is installed in place of reflect.callReflect.
// It recognizes our closures and prints args; otherwise it delegates to the original callReflect.
func callDump(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	if DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] callDump entry ctxt=%p frame=%p regs=%p\n", ctxt, frame, regs)
	}
	if ctxt != nil {
		if DebugEnabled {
			first := *(*uintptr)(ctxt)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump] ctxt.firstWord=0x%x (makeFuncStub=0x%x)\n", first, DebugLastMakeFuncStubPtr)
		}
		impl := (*dumpFuncImpl)(ctxt)
		if DebugEnabled {
			// Only print fields that cannot trigger reflect on corrupted memory.
			_, _ = fmt.Fprintf(os.Stderr, "[argdump] magic=0x%x expect=0x%x\n",
				impl.magic, uintptr(unsafe.Pointer(magicSentinel)),
			)
		}
		if impl.magic == uintptr(unsafe.Pointer(magicSentinel)) && impl.fnType != nil {
			dumpArgsAndZeroRets(impl, frame, retValid, regs)
			return
		}
	}
	if origCallReflect == nil {
		panic("argdump: original callReflect trampoline not initialized")
	}
	origCallReflect(ctxt, frame, retValid, regs)
}

func dumpArgsAndZeroRets(impl *dumpFuncImpl, frame unsafe.Pointer, retValid *bool, regsPtr unsafe.Pointer) {
	regs := (*regArgs)(regsPtr)

	opt := abijson.DefaultOptions()
	opt.MaxDepth = 3

	// Dump inputs.
	var keepAlive []any
	if DumpEnabled {
		for i := 0; i < impl.fnType.NumIn(); i++ {
			at := impl.fnType.In(i)
			if at.Size() == 0 {
				fmt.Printf("arg%d=%s\n", i, "null")
				continue
			}

			addr, ka := impl.abid.addrOfArg(i, at, frame, regs)
			if ka != nil {
				keepAlive = append(keepAlive, ka)
			}
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

	// If we're being used as a function hook, call the original function and let it
	// write return values back into frame/regs.
	if impl.hookGuard != nil && impl.origFuncVal != nil {
		if DebugEnabled {
			// Minimal, high-signal checkpoints. Avoid large formatting.
			_, _ = fmt.Fprintf(os.Stderr,
				"[argdump] hook enter frame=%p regs=%p origFuncVal=%p argsSz=%d retOff=%d frameSz=%d\n",
				frame, regs, impl.origFuncVal, impl.stackArgsSz, impl.stackRetOff, impl.frameSize,
			)
			// Read the first word of func value (code ptr) and a few bytes at that address.
			codePtr := *(*uintptr)(impl.origFuncVal)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump] orig codePtr=0x%x\n", codePtr)
			if codePtr != 0 {
				if runtime.FuncForPC(codePtr) == nil {
					_, _ = fmt.Fprintf(os.Stderr, "[argdump] ERROR: codePtr not in any known function (outside text?): 0x%x\n", codePtr)
					panic("argdump: orig code ptr not in text (FuncForPC returned nil)")
				}
			}
			if codePtr != 0 {
				// Best-effort read; if this fails, the process will likely panic anyway.
				b := memory.RawRead(codePtr, 16)
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] orig code first16=%x\n", b)
			}
		}

		// Two modes:
		// - trampoline mode: call the "fixed origin" (copied+relocated prologue) directly, without unpatching.
		// - fallback mode (closures/method values): temporarily unpatch origin to avoid recursion, then re-apply patch.
		if !impl.useTrampoline {
			impl.hookGuard.UnpatchWithLock()
			// IMPORTANT: Guard.Restore() means "re-apply patch bytes", NOT "restore original bytes".
			// We defer it to ensure PatchFunc remains active even if the original function panics.
			defer impl.hookGuard.Restore()
		}
		defer func() {
			if r := recover(); r != nil {
				// Dump panic info (if enabled), then re-panic to preserve semantics.
				if DumpEnabled {
					dumpPanic(r, opt)
				}
				panic(r)
			}
		}()
		if DebugEnabled {
			if impl.useTrampoline {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] calling runtime.reflectcall via trampoline=0x%x\n", impl.hookGuard.FixOriginFunc())
			} else {
				_, _ = fmt.Fprintln(os.Stderr, "[argdump] unpatched; calling runtime.reflectcall")
			}
		}
		regs.ReturnIsPtr = impl.abid.outRegPtrs
		runtimeReflectcall(
			nil,
			impl.origFuncVal,
			frame,
			impl.stackArgsSz,
			impl.stackRetOff,
			impl.frameSize,
			regs,
		)
		if DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] reflectcall returned; restoring patch")
		}

		// Dump outputs produced by the original function.
		if DumpEnabled {
			dumpReturns(impl, frame, regs, opt)
		}
		if DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] returning to caller (patch restore deferred)")
		}
		// Only mark stack return space valid if we actually have stack return bytes.
		// Otherwise the runtime may scan uninitialized return slots and treat garbage as pointers.
		if retValid != nil && impl.stackArgsSz > impl.stackRetOff {
			*retValid = true
		}
		// Make sure any temporary values materialized from register args/returns remain alive
		// for the duration of JSON encoding. (Their addresses are held only in unsafe pointers.)
		runtime.KeepAlive(keepAlive)
		return
	}

	// Otherwise (standalone dump func), return zero values.
	impl.abid.zeroRets(impl.fnType, frame, regs)
	if retValid != nil && impl.abid.stackCallArgsSize > uintptr(impl.abid.retOffset) {
		*retValid = true
	}
	runtime.KeepAlive(keepAlive)
}

func dumpReturns(impl *dumpFuncImpl, frame unsafe.Pointer, regs *regArgs, opt abijson.Options) {
	var keepAlive []any
	for i := 0; i < impl.fnType.NumOut(); i++ {
		rt := impl.fnType.Out(i)
		if rt.Size() == 0 {
			fmt.Printf("ret%d=%s\n", i, "null")
			continue
		}
		addr, ka := impl.abid.addrOfRet(i, rt, frame, regs)
		if ka != nil {
			keepAlive = append(keepAlive, ka)
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
	runtime.KeepAlive(keepAlive)
}

func dumpPanic(v interface{}, _ abijson.Options) {
	// Do NOT attempt to pass &v to abijson with reflect.TypeOf(v):
	// &v points to an interface header, not the concrete value storage.
	//
	// For panic dumping we only need the message; stringify safely and JSON-quote it.
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
func runtimeReflectcall(stackArgsType unsafe.Pointer, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *regArgs)

// --- ABI layout (reflect/abi.go-derived; uses reflect.Type, not reflect.Value) ---

type abiStepKind int

const (
	abiStepBad abiStepKind = iota
	abiStepStack
	abiStepIntReg
	abiStepPointer
	abiStepFloatReg
)

type abiStep struct {
	kind abiStepKind

	offset uintptr
	size   uintptr

	stkOff uintptr
	ireg   int
	freg   int
}

type abiSeq struct {
	steps      []abiStep
	valueStart []int

	stackBytes   uintptr
	iregs, fregs int
}

func (a *abiSeq) stepsForValue(i int) []abiStep {
	s := a.valueStart[i]
	var e int
	if i == len(a.valueStart)-1 {
		e = len(a.steps)
	} else {
		e = a.valueStart[i+1]
	}
	return a.steps[s:e]
}

func (a *abiSeq) addArg(t reflect.Type) *abiStep {
	pStart := len(a.steps)
	a.valueStart = append(a.valueStart, pStart)
	if t.Size() == 0 {
		a.stackBytes = align(a.stackBytes, uintptr(t.Align()))
		return nil
	}

	aOld := *a
	if !a.regAssign(t, 0) {
		*a = aOld
		a.stackAssign(t.Size(), uintptr(t.Align()))
		return &a.steps[len(a.steps)-1]
	}
	return nil
}

func (a *abiSeq) regAssign(t reflect.Type, offset uintptr) bool {
	switch t.Kind() {
	case reflect.UnsafePointer, kindPointer, reflect.Chan, reflect.Map, reflect.Func:
		return a.assignIntN(offset, t.Size(), 1, 0b1)
	case reflect.Bool, reflect.Int, reflect.Uint, reflect.Int8, reflect.Uint8, reflect.Int16, reflect.Uint16,
		reflect.Int32, reflect.Uint32, reflect.Uintptr:
		return a.assignIntN(offset, t.Size(), 1, 0b0)
	case reflect.Int64, reflect.Uint64:
		return a.assignIntN(offset, 8, 1, 0b0)
	case reflect.Float32, reflect.Float64:
		return a.assignFloatN(offset, t.Size(), 1)
	case reflect.Complex64:
		return a.assignFloatN(offset, 4, 2)
	case reflect.Complex128:
		return a.assignFloatN(offset, 8, 2)
	case reflect.String:
		return a.assignIntN(offset, ptrSize, 2, 0b01)
	case reflect.Interface:
		// Follow reflect/abi.go: treat the type/itab word as non-heap pointer.
		return a.assignIntN(offset, ptrSize, 2, 0b10)
	case reflect.Slice:
		return a.assignIntN(offset, ptrSize, 3, 0b001)
	case reflect.Array:
		switch t.Len() {
		case 0:
			return true
		case 1:
			return a.regAssign(t.Elem(), offset)
		default:
			return false
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !a.regAssign(f.Type, offset+uintptr(f.Offset)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (a *abiSeq) assignIntN(offset, size uintptr, n int, ptrMap uint8) bool {
	if n > 8 || n < 0 {
		panic("argdump: invalid n")
	}
	if ptrMap != 0 && size != ptrSize {
		panic("argdump: ptrMap for non-pointer-size values")
	}
	if a.iregs+n > intArgRegs {
		return false
	}
	for i := 0; i < n; i++ {
		kind := abiStepIntReg
		if ptrMap&(uint8(1)<<i) != 0 {
			kind = abiStepPointer
		}
		a.steps = append(a.steps, abiStep{
			kind:   kind,
			offset: offset + uintptr(i)*size,
			size:   size,
			ireg:   a.iregs,
		})
		a.iregs++
	}
	return true
}

func (a *abiSeq) assignFloatN(offset, size uintptr, n int) bool {
	if n < 0 {
		panic("argdump: invalid n")
	}
	if a.fregs+n > floatArgRegs || floatRegSize < size {
		return false
	}
	for i := 0; i < n; i++ {
		a.steps = append(a.steps, abiStep{
			kind:   abiStepFloatReg,
			offset: offset + uintptr(i)*size,
			size:   size,
			freg:   a.fregs,
		})
		a.fregs++
	}
	return true
}

func (a *abiSeq) stackAssign(size, alignment uintptr) {
	a.stackBytes = align(a.stackBytes, alignment)
	a.steps = append(a.steps, abiStep{
		kind:   abiStepStack,
		offset: 0,
		size:   size,
		stkOff: a.stackBytes,
	})
	a.stackBytes += size
}

type abiDesc struct {
	call, ret abiSeq

	stackCallArgsSize uintptr
	retOffset         uintptr
	spill             uintptr

	stackPtrs *bitVector

	inRegPtrs  intArgRegBitmap
	outRegPtrs intArgRegBitmap
}

func newAbiDescFromFuncType(t reflect.Type) abiDesc {
	spill := uintptr(0)
	stackPtrs := new(bitVector)
	inRegPtrs := intArgRegBitmap{}

	var in abiSeq
	for i := 0; i < t.NumIn(); i++ {
		arg := t.In(i)
		stkStep := in.addArg(arg)
		if stkStep != nil {
			addTypeBits(stackPtrs, stkStep.stkOff, arg)
		} else {
			spill = align(spill, uintptr(arg.Align()))
			spill += arg.Size()
			for _, st := range in.stepsForValue(i) {
				if st.kind == abiStepPointer {
					inRegPtrs.Set(st.ireg)
				}
			}
		}
	}
	spill = align(spill, ptrSize)

	stackCallArgsSize := in.stackBytes
	retOffset := align(in.stackBytes, ptrSize)

	outRegPtrs := intArgRegBitmap{}

	var out abiSeq
	out.stackBytes = retOffset
	for i := 0; i < t.NumOut(); i++ {
		res := t.Out(i)
		stkStep := out.addArg(res)
		if stkStep != nil {
			addTypeBits(stackPtrs, stkStep.stkOff, res)
		} else {
			for _, st := range out.stepsForValue(i) {
				if st.kind == abiStepPointer {
					outRegPtrs.Set(st.ireg)
				}
			}
		}
	}
	out.stackBytes -= retOffset

	return abiDesc{
		call:              in,
		ret:               out,
		stackCallArgsSize: stackCallArgsSize,
		retOffset:         retOffset,
		spill:             spill,
		stackPtrs:         stackPtrs,
		inRegPtrs:         inRegPtrs,
		outRegPtrs:        outRegPtrs,
	}
}

func (a abiDesc) frameSizeBytes() uintptr {
	// Total stackArgs area size = retOffset + stack return bytes.
	// reflectcall uses this to select call{N} implementation and needs enough
	// extra space to spill register arguments for preemption/GC safety.
	return align(a.retOffset+a.ret.stackBytes+a.spill, ptrSize)
}

func (a abiDesc) frameTypeSizeBytes() uintptr {
	// Size of the argument frame type passed as stackArgsType/stackArgsSize to reflectcall:
	// stack args + stack results area (no spill).
	return align(a.retOffset+a.ret.stackBytes, ptrSize)
}

func (a abiDesc) addrOfArg(i int, t reflect.Type, frame unsafe.Pointer, regs *regArgs) (unsafe.Pointer, any) {
	steps := a.call.stepsForValue(i)
	if len(steps) == 0 {
		return nil, nil
	}
	if steps[0].kind == abiStepStack {
		addr := unsafe.Add(frame, steps[0].stkOff)
		if DebugEnabled && i == 0 && t.Kind() == reflect.String {
			sh := (*reflect.StringHeader)(addr)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump][debug] arg0 string from STACK off=%d data=0x%x len=%d\n", steps[0].stkOff, sh.Data, sh.Len)
		}
		return addr, nil
	}

	// Register-passed args need an addressable backing store for EncodeJSONFromAddr.
	//
	// IMPORTANT: We must keep the backing store alive using a normal Go reference.
	// Returning only an unsafe.Pointer is not enough: JSON encoding may allocate and
	// trigger GC, which can reclaim the temp object if it isn't otherwise referenced.
	if t.Size() == 0 {
		return nil, nil
	}
	box := reflect.New(t) // *T
	dst := unsafe.Pointer(box.Pointer())
	if DebugEnabled && i == 0 && t.Kind() == reflect.String {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump][debug] arg0 string from REGS steps=%v ints0=0x%x ints1=0x%x ptr0=%p ptr1=%p\n",
			steps, regs.Ints[0], regs.Ints[1], regs.Ptrs[0], regs.Ptrs[1],
		)
	}
	for _, st := range steps {
		switch st.kind {
		case abiStepIntReg:
			memmove(unsafe.Add(dst, st.offset), regs.intRegArgAddr(st.ireg, st.size), st.size)
		case abiStepPointer:
			// For pointer-carrying regs, prefer RegArgs.Ptrs (GC-visible pointer-typed slots).
			// This avoids depending on the untyped Ints view for pointers, which can be
			// brittle across versions/toolchains.
			memmove(unsafe.Add(dst, st.offset), unsafe.Pointer(&regs.Ptrs[st.ireg]), st.size)
		case abiStepFloatReg:
			switch st.size {
			case 4:
				f := archFloat32FromReg(regs.Floats[st.freg])
				*(*float32)(unsafe.Add(dst, st.offset)) = f
			case 8:
				*(*uint64)(unsafe.Add(dst, st.offset)) = regs.Floats[st.freg]
			default:
				panic("argdump: bad float size")
			}
		default:
			panic("argdump: unknown step kind")
		}
	}
	return dst, box.Interface()
}

func (a abiDesc) addrOfRet(i int, t reflect.Type, frame unsafe.Pointer, regs *regArgs) (unsafe.Pointer, any) {
	steps := a.ret.stepsForValue(i)
	if len(steps) == 0 {
		return nil, nil
	}
	if steps[0].kind == abiStepStack {
		return unsafe.Add(frame, steps[0].stkOff), nil
	}

	// Register-returned values need an addressable backing store for EncodeJSONFromAddr.
	// See addrOfArg for the GC/keepalive rationale.
	if t.Size() == 0 {
		return nil, nil
	}
	box := reflect.New(t) // *T
	dst := unsafe.Pointer(box.Pointer())
	for _, st := range steps {
		switch st.kind {
		case abiStepIntReg:
			memmove(unsafe.Add(dst, st.offset), regs.intRegArgAddr(st.ireg, st.size), st.size)
		case abiStepPointer:
			memmove(unsafe.Add(dst, st.offset), unsafe.Pointer(&regs.Ptrs[st.ireg]), st.size)
		case abiStepFloatReg:
			switch st.size {
			case 4:
				f := archFloat32FromReg(regs.Floats[st.freg])
				*(*float32)(unsafe.Add(dst, st.offset)) = f
			case 8:
				*(*uint64)(unsafe.Add(dst, st.offset)) = regs.Floats[st.freg]
			default:
				panic("argdump: bad float size")
			}
		default:
			panic("argdump: unknown step kind")
		}
	}
	return dst, box.Interface()
}

func (a abiDesc) zeroRets(fnType reflect.Type, frame unsafe.Pointer, regs *regArgs) {
	for i := 0; i < fnType.NumOut(); i++ {
		rt := fnType.Out(i)
		if rt.Size() == 0 {
			continue
		}
		steps := a.ret.stepsForValue(i)
		for _, st := range steps {
			switch st.kind {
			case abiStepStack:
				memclr(unsafe.Add(frame, st.stkOff), st.size)
			case abiStepIntReg, abiStepPointer:
				memclr(regs.intRegArgAddr(st.ireg, st.size), st.size)
			case abiStepFloatReg:
				regs.Floats[st.freg] = 0
			default:
				panic("argdump: unknown step kind")
			}
		}
	}
}

func memmove(to, from unsafe.Pointer, n uintptr) {
	if n == 0 {
		return
	}
	ni := int(n)
	if uintptr(ni) != n {
		panic("argdump: memmove size overflow")
	}
	copy(unsafe.Slice((*byte)(to), ni), unsafe.Slice((*byte)(from), ni))
}


func archFloat32FromReg(reg uint64) float32 {
	i := uint32(reg)
	return *(*float32)(unsafe.Pointer(&i))
}
