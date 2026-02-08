//go:build go1.18 && arm64
// +build go1.18,arm64

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
	OrigFuncValBox anyCompat
	UseTrampoline  bool
	StackArgsSz    uint32
	StackRetOff    uint32
	FrameSize      uint32
}

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

type abiDesc struct {
	call, ret abiSeq

	stackCallArgsSize uintptr
	retOffset         uintptr
	spill             uintptr

	stackPtrs *bitvec.BitVector

	inRegPtrs  intArgRegBitmap
	outRegPtrs intArgRegBitmap
}
