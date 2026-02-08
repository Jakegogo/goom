//go:build (go1.17 && !go1.18 && amd64) || (go1.18 && amd64)
// +build go1.17,!go1.18,amd64 go1.18,amd64

package regabi

import (
	"reflect"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/impl/shared"
	"github.com/tencent/goom/internal/argdump/internal/bitvec"
	"github.com/tencent/goom/internal/patch"
)

// intArgRegBitmap is an alias for shared.IntArgRegBitmap.
type intArgRegBitmap = shared.IntArgRegBitmap

// regArgs matches internal/abi.RegArgs (layout-sensitive).
// This structure must exactly match the layout of internal/abi.RegArgs
// in the Go runtime for the target version.
type regArgs struct {
	Ints   [abi.IntArgRegs]uintptr        // Integer argument registers
	Floats [abi.FloatArgRegs]uint64       // Float argument registers
	Ptrs   [abi.IntArgRegs]unsafe.Pointer // Pointer map for GC
	ReturnIsPtr intArgRegBitmap            // Bitmap for return value pointers
}

func (r *regArgs) intRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
	return unsafe.Pointer(&r.Ints[reg])
}

// makeFuncCtxt matches reflect.makeFuncCtxt prefix (layout-sensitive).
type makeFuncCtxt struct {
	fn      uintptr           // Function pointer to reflect.makeFuncStub
	stack   *bitvec.BitVector // Stack pointer bitmap for GC
	argLen  uintptr           // Total argument length
	regPtrs intArgRegBitmap   // Register pointer bitmap
}

// DumpFuncImpl is the dump function implementation.
type DumpFuncImpl struct {
	makeFuncCtxt

	magic uintptr // Magic sentinel for type checking

	fnType reflect.Type // Function type being wrapped
	abid   abiDesc      // ABI descriptor

	HookGuard      *patch.Guard  // Patch guard for original function
	OrigFuncVal    unsafe.Pointer
	OrigFuncValBox anyCompat
	UseTrampoline  bool
	StackArgsSz    uint32 // Stack arguments size
	StackRetOff    uint32 // Stack return offset
	FrameSize      uint32 // Total frame size
}

// --- ABI layout types ---

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

type abiDesc struct {
	call, ret abiSeq

	stackCallArgsSize uintptr
	retOffset         uintptr
	spill             uintptr

	stackPtrs *bitvec.BitVector

	inRegPtrs  intArgRegBitmap
	outRegPtrs intArgRegBitmap
}
