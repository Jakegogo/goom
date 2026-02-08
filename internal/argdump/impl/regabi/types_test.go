//go:build (go1.17 && !go1.18 && amd64) || (go1.18 && amd64)
// +build go1.17,!go1.18,amd64 go1.18,amd64

package regabi

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
)

func TestBuildTagCorrectness(t *testing.T) {
	ver := runtime.Version()
	t.Logf("Running on Go version: %s", ver)

	// Verify we're on the expected architecture
	if runtime.GOARCH != "amd64" {
		t.Fatalf("Expected amd64, got %s", runtime.GOARCH)
	}

	// Verify the right compat type is selected
	var x anyCompat
	_ = x

	t.Logf("anyCompat type: %T", x)
}

func TestTypeLayoutConsistency(t *testing.T) {
	// Verify regArgs layout matches expectations
	var r regArgs

	if size := unsafe.Sizeof(r); size == 0 {
		t.Fatal("regArgs size is zero")
	}

	t.Logf("regArgs size: %d bytes", unsafe.Sizeof(r))
	t.Logf("  Ints offset: %d, size: %d", unsafe.Offsetof(r.Ints), unsafe.Sizeof(r.Ints))
	t.Logf("  Floats offset: %d, size: %d", unsafe.Offsetof(r.Floats), unsafe.Sizeof(r.Floats))
	t.Logf("  Ptrs offset: %d, size: %d", unsafe.Offsetof(r.Ptrs), unsafe.Sizeof(r.Ptrs))
	t.Logf("  ReturnIsPtr offset: %d, size: %d", unsafe.Offsetof(r.ReturnIsPtr), unsafe.Sizeof(r.ReturnIsPtr))

	// Verify expected field counts
	expectedInts := abi.IntArgRegs
	expectedFloats := abi.FloatArgRegs
	t.Logf("  IntArgRegs: %d, FloatArgRegs: %d", expectedInts, expectedFloats)

	// Verify makeFuncCtxt layout
	var m makeFuncCtxt
	t.Logf("makeFuncCtxt size: %d bytes", unsafe.Sizeof(m))
	t.Logf("  fn offset: %d", unsafe.Offsetof(m.fn))
	t.Logf("  stack offset: %d", unsafe.Offsetof(m.stack))
	t.Logf("  argLen offset: %d", unsafe.Offsetof(m.argLen))
	t.Logf("  regPtrs offset: %d", unsafe.Offsetof(m.regPtrs))

	// Verify DumpFuncImpl layout
	var d DumpFuncImpl
	t.Logf("DumpFuncImpl size: %d bytes", unsafe.Sizeof(d))
	t.Logf("  makeFuncCtxt offset: %d, size: %d", unsafe.Offsetof(d.makeFuncCtxt), unsafe.Sizeof(d.makeFuncCtxt))
	t.Logf("  magic offset: %d", unsafe.Offsetof(d.magic))
	t.Logf("  fnType offset: %d", unsafe.Offsetof(d.fnType))
	t.Logf("  abid offset: %d", unsafe.Offsetof(d.abid))
}

func TestAbiStepKindValues(t *testing.T) {
	// Verify enum values are as expected
	if abiStepBad != 0 {
		t.Errorf("abiStepBad = %d, want 0", abiStepBad)
	}
	if abiStepStack != 1 {
		t.Errorf("abiStepStack = %d, want 1", abiStepStack)
	}
	if abiStepIntReg != 2 {
		t.Errorf("abiStepIntReg = %d, want 2", abiStepIntReg)
	}
	if abiStepPointer != 3 {
		t.Errorf("abiStepPointer = %d, want 3", abiStepPointer)
	}
	if abiStepFloatReg != 4 {
		t.Errorf("abiStepFloatReg = %d, want 4", abiStepFloatReg)
	}
}

func TestRegArgsIntRegArgAddr(t *testing.T) {
	var r regArgs
	r.Ints[0] = 0x1234567890ABCDEF

	addr := r.intRegArgAddr(0, 8)
	if addr == nil {
		t.Fatal("intRegArgAddr returned nil")
	}

	val := *(*uintptr)(addr)
	if val != 0x1234567890ABCDEF {
		t.Errorf("Expected 0x1234567890ABCDEF, got 0x%x", val)
	}
}

func TestAbiSeqStepsForValue(t *testing.T) {
	seq := abiSeq{
		steps: []abiStep{
			{kind: abiStepIntReg, ireg: 0},
			{kind: abiStepIntReg, ireg: 1},
			{kind: abiStepStack, stkOff: 0},
		},
		valueStart: []int{0, 2},
	}

	// First value should have steps[0:2]
	steps := seq.stepsForValue(0)
	if len(steps) != 2 {
		t.Errorf("Expected 2 steps for value 0, got %d", len(steps))
	}

	// Second value should have steps[2:3]
	steps = seq.stepsForValue(1)
	if len(steps) != 1 {
		t.Errorf("Expected 1 step for value 1, got %d", len(steps))
	}
	if steps[0].kind != abiStepStack {
		t.Errorf("Expected abiStepStack, got %v", steps[0].kind)
	}
}
