//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

package go118

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/abijson"
	"github.com/tencent/goom/internal/argdump/impl/shared"
	"github.com/tencent/goom/internal/argdump/internal/bitvec"
	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/argdump/internal/mem"
	"github.com/tencent/goom/internal/argdump/internal/util"
	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	_ "github.com/tencent/goom/internal/bytecode/stub" // ensure stub symbols exist for //go:linkname users (icache).
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// intArgRegBitmap is an alias for shared.IntArgRegBitmap.
type intArgRegBitmap = shared.IntArgRegBitmap

// regArgs matches internal/abi.RegArgs (layout-sensitive).
type regArgs struct {
	Ints   [abi.IntArgRegs]uintptr
	Floats [abi.FloatArgRegs]uint64

	Ptrs [abi.IntArgRegs]unsafe.Pointer

	ReturnIsPtr intArgRegBitmap
}

func (r *regArgs) intRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
	return unsafe.Pointer(&r.Ints[reg])
}

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive).
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
	callDumpFuncVal func(ctxt unsafe.Pointer, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer)

	magicSentinel = new(int)
)

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
			regPtrs: abid.inRegPtrs,
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	return util.PackEface(util.RtypePtr(typ), unsafe.Pointer(impl))
}

func ensureCallReflectPatched() {
	patchOnce.Do(func() {
		callReflectPtr := uintptr(0)
		if callReflectAbi0Ptr, e := unexports2.FindFuncByName("reflect.callReflect.abi0"); e == nil && callReflectAbi0Ptr != 0 {
			if inner, e2 := bytecode.GetInnerFunc(0, callReflectAbi0Ptr); e2 == nil && inner != 0 && inner != callReflectAbi0Ptr {
				callReflectPtr = inner
			}
			if flags.DebugEnabled {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] resolve callReflect.abi0=0x%x inner=0x%x\n", callReflectAbi0Ptr, callReflectPtr)
			}
		}
		if callReflectPtr == 0 {
			p, err := unexports2.FindFuncByName("reflect.callReflect")
			if err != nil {
				panic(err)
			}
			callReflectPtr = p
			if flags.DebugEnabled {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] resolve callReflect=0x%x\n", p)
			}
		}
		flags.DebugCallReflectPtr = callReflectPtr
		flags.DebugCallDumpPtr = reflect.ValueOf(callDump).Pointer()
		flags.DebugRuntimeReflectcallPtr = reflect.ValueOf(runtimeReflectcall).Pointer()

		if p, e := unexports2.FindFuncByName("runtime.spillArgs"); e == nil {
			flags.DebugRuntimeSpillArgsPtr = p
		}
		if p, e := unexports2.FindFuncByName("runtime.unspillArgs"); e == nil {
			flags.DebugRuntimeUnspillArgsPtr = p
		}
		if p, e := unexports2.FindFuncByName("reflect.moveMakeFuncArgPtrs"); e == nil {
			flags.DebugMoveMakeFuncArgPtrsPtr = p
		}

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

		_, err = unexports2.CreateFuncForCodePtr(&tmp, guard.FixOriginFunc())
		if err != nil {
			panic(err)
		}
		origCallReflect = tmp

		guard.Apply()
	})
}

// callReflectTrampolineHolder is an alias for the shared trampoline holder.
var callReflectTrampolineHolder = shared.CallReflectTrampolineHolder

func callDump(ctxt, frame unsafe.Pointer, retValid *bool, regs unsafe.Pointer) {
	if flags.DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] callDump entry ctxt=%p frame=%p regs=%p\n", ctxt, frame, regs)
	}
	if ctxt != nil {
		if flags.DebugEnabled {
			first := *(*uintptr)(ctxt)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump] ctxt.firstWord=0x%x (makeFuncStub=0x%x)\n", first, flags.DebugLastMakeFuncStubPtr)
		}
		impl := (*DumpFuncImpl)(ctxt)
		if flags.DebugEnabled {
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

func dumpArgsAndZeroRets(impl *DumpFuncImpl, frame unsafe.Pointer, retValid *bool, regsPtr unsafe.Pointer) {
	regs := (*regArgs)(regsPtr)

	opt := abijson.DefaultOptions()
	opt.MaxDepth = 3

	var keepAlive []any
	if flags.DumpEnabled {
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

	if impl.HookGuard != nil && impl.OrigFuncVal != nil {
		if flags.DebugEnabled {
			_, _ = fmt.Fprintf(os.Stderr,
				"[argdump] hook enter frame=%p regs=%p origFuncVal=%p argsSz=%d retOff=%d frameSz=%d\n",
				frame, regs, impl.OrigFuncVal, impl.StackArgsSz, impl.StackRetOff, impl.FrameSize,
			)
			codePtr := *(*uintptr)(impl.OrigFuncVal)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump] orig codePtr=0x%x\n", codePtr)
			if codePtr != 0 {
				if runtime.FuncForPC(codePtr) == nil {
					_, _ = fmt.Fprintf(os.Stderr, "[argdump] ERROR: codePtr not in any known function (outside text?): 0x%x\n", codePtr)
					panic("argdump: orig code ptr not in text (FuncForPC returned nil)")
				}
			}
			if codePtr != 0 {
				b := memory.RawRead(codePtr, 16)
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] orig code first16=%x\n", b)
			}
		}

		if !impl.UseTrampoline {
			impl.HookGuard.UnpatchWithLock()
			defer impl.HookGuard.Restore()
		}
		defer func() {
			if r := recover(); r != nil {
				if flags.DumpEnabled {
					dumpPanic(r, opt)
				}
				panic(r)
			}
		}()
		if flags.DebugEnabled {
			if impl.UseTrampoline {
				_, _ = fmt.Fprintf(os.Stderr, "[argdump] calling runtime.reflectcall via trampoline=0x%x\n", impl.HookGuard.FixOriginFunc())
			} else {
				_, _ = fmt.Fprintln(os.Stderr, "[argdump] unpatched; calling runtime.reflectcall")
			}
		}
		regs.ReturnIsPtr = impl.abid.outRegPtrs
		runtimeReflectcall(
			nil,
			impl.OrigFuncVal,
			frame,
			impl.StackArgsSz,
			impl.StackRetOff,
			impl.FrameSize,
			regs,
		)
		if flags.DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] reflectcall returned; restoring patch")
		}

		if flags.DumpEnabled {
			dumpReturns(impl, frame, regs, opt)
		}
		if flags.DebugEnabled {
			_, _ = fmt.Fprintln(os.Stderr, "[argdump] returning to caller (patch restore deferred)")
		}
		if retValid != nil && impl.StackArgsSz > impl.StackRetOff {
			*retValid = true
		}
		runtime.KeepAlive(keepAlive)
		return
	}

	impl.abid.zeroRets(impl.fnType, frame, regs)
	if retValid != nil && impl.abid.stackCallArgsSize > uintptr(impl.abid.retOffset) {
		*retValid = true
	}
	runtime.KeepAlive(keepAlive)
}

func dumpReturns(impl *DumpFuncImpl, frame unsafe.Pointer, regs *regArgs, opt abijson.Options) {
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
	shared.DumpPanic(v)
}

//go:linkname runtimeReflectcall runtime.reflectcall
func runtimeReflectcall(stackArgsType unsafe.Pointer, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *regArgs)

// --- ABI layout ---

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
		a.stackBytes = util.Align(a.stackBytes, uintptr(t.Align()))
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
	case reflect.UnsafePointer, abi.KindPointer, reflect.Chan, reflect.Map, reflect.Func:
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
		return a.assignIntN(offset, abi.PtrSize, 2, 0b01)
	case reflect.Interface:
		return a.assignIntN(offset, abi.PtrSize, 2, 0b10)
	case reflect.Slice:
		return a.assignIntN(offset, abi.PtrSize, 3, 0b001)
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
	if ptrMap != 0 && size != abi.PtrSize {
		panic("argdump: ptrMap for non-pointer-size values")
	}
	if a.iregs+n > abi.IntArgRegs {
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
	if a.fregs+n > abi.FloatArgRegs || abi.FloatRegSize < size {
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
	a.stackBytes = util.Align(a.stackBytes, alignment)
	a.steps = append(a.steps, abiStep{
		kind:   abiStepStack,
		offset: 0,
		size:   size,
		stkOff: a.stackBytes,
	})
	a.stackBytes += size
}

// nolint
type abiDesc struct {
	call, ret abiSeq

	stackCallArgsSize uintptr
	retOffset         uintptr
	spill             uintptr

	stackPtrs *bitvec.BitVector

	inRegPtrs  intArgRegBitmap
	outRegPtrs intArgRegBitmap
}

func newAbiDescFromFuncType(t reflect.Type) abiDesc {
	spill := uintptr(0)
	stackPtrs := new(bitvec.BitVector)
	inRegPtrs := intArgRegBitmap{}

	var in abiSeq
	for i := 0; i < t.NumIn(); i++ {
		arg := t.In(i)
		stkStep := in.addArg(arg)
		if stkStep != nil {
			util.AddTypeBits(stackPtrs, stkStep.stkOff, arg)
		} else {
			spill = util.Align(spill, uintptr(arg.Align()))
			spill += arg.Size()
			for _, st := range in.stepsForValue(i) {
				if st.kind == abiStepPointer {
					inRegPtrs.Set(st.ireg)
				}
			}
		}
	}
	spill = util.Align(spill, abi.PtrSize)

	stackCallArgsSize := in.stackBytes
	retOffset := util.Align(in.stackBytes, abi.PtrSize)

	outRegPtrs := intArgRegBitmap{}

	var out abiSeq
	out.stackBytes = retOffset
	for i := 0; i < t.NumOut(); i++ {
		res := t.Out(i)
		stkStep := out.addArg(res)
		if stkStep != nil {
			util.AddTypeBits(stackPtrs, stkStep.stkOff, res)
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
	return util.Align(a.retOffset+a.ret.stackBytes+a.spill, abi.PtrSize)
}

func (a abiDesc) frameTypeSizeBytes() uintptr {
	return util.Align(a.retOffset+a.ret.stackBytes, abi.PtrSize)
}

func (a abiDesc) addrOfArg(i int, t reflect.Type, frame unsafe.Pointer, regs *regArgs) (unsafe.Pointer, any) {
	steps := a.call.stepsForValue(i)
	if len(steps) == 0 {
		return nil, nil
	}
	if steps[0].kind == abiStepStack {
		addr := unsafe.Add(frame, steps[0].stkOff)
		if flags.DebugEnabled && i == 0 && t.Kind() == reflect.String {
			sh := (*reflect.StringHeader)(addr)
			_, _ = fmt.Fprintf(os.Stderr, "[argdump][debug] arg0 string from STACK off=%d data=0x%x len=%d\n", steps[0].stkOff, sh.Data, sh.Len)
		}
		return addr, nil
	}

	if t.Size() == 0 {
		return nil, nil
	}
	box := reflect.New(t)
	dst := unsafe.Pointer(box.Pointer())
	if flags.DebugEnabled && i == 0 && t.Kind() == reflect.String {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump][debug] arg0 string from REGS steps=%v ints0=0x%x ints1=0x%x ptr0=%p ptr1=%p\n",
			steps, regs.Ints[0], regs.Ints[1], regs.Ptrs[0], regs.Ptrs[1],
		)
	}
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

func (a abiDesc) addrOfRet(i int, t reflect.Type, frame unsafe.Pointer, regs *regArgs) (unsafe.Pointer, any) {
	steps := a.ret.stepsForValue(i)
	if len(steps) == 0 {
		return nil, nil
	}
	if steps[0].kind == abiStepStack {
		return unsafe.Add(frame, steps[0].stkOff), nil
	}

	if t.Size() == 0 {
		return nil, nil
	}
	box := reflect.New(t)
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
				mem.Memclr(unsafe.Add(frame, st.stkOff), st.size)
			case abiStepIntReg, abiStepPointer:
				mem.Memclr(regs.intRegArgAddr(st.ireg, st.size), st.size)
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
