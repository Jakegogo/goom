//go:build arm64
// +build arm64

package patch

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/tencent/goom/internal/logger"
)

// CtxtBranchJumpSize is the size in bytes of jmpToCodeWithCtxBranch (arm64).
const CtxtBranchJumpSize = 20

// PtrFuncVal patches originPtr to jump to a *func value* located at funcVal.
// Unlike Ptr/Trampoline, this is specialized for func values (closures) and always
// sets the arm64 context register (x26) to funcVal before branching.
//
// If fixOriginPtr is non-zero, it is recorded in the returned Guard for callers
// that need to call the original function.
func PtrFuncVal(originPtr uintptr, funcVal unsafe.Pointer, fixOriginPtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	// Unify jumps: always set x26=ctxt, then jump to a code address (no LDR [x26]).
	// For func values, the ctxt is the func value pointer, and the code pointer is its first word.
	funcValPtr := uintptr(funcVal)
	codePtr := *(*uintptr)(funcVal)
	jumpBytes, err := jmpToCodeWithCtxJump(originPtr, funcValPtr, codePtr)
	if err != nil {
		return nil, err
	}
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrFuncVal origin=0x%x funcValPtr=0x%x code=0x%x jumpLen=%d", originPtr, funcValPtr, codePtr, len(jumpBytes))
	return guard, nil
}

// PtrCodeWithCtx patches originPtr to set x26=ctxtPtr then branch directly to codePtr.
// This avoids loading an (arm64e) pointer-authenticated code pointer from memory.
//
// It emits:
//
//	MOVZ/MOVK x26, ctxtPtr
//	B codePtr
func PtrCodeWithCtx(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr, fixOriginPtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	jumpBytes, err := jmpToCodeWithCtxJump(originPtr, ctxtPtr, codePtr)
	if err != nil {
		return nil, err
	}
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrCodeWithCtx origin=0x%x ctxt=0x%x code=0x%x jumpLen=%d", originPtr, ctxtPtr, codePtr, len(jumpBytes))
	return guard, nil
}

// PtrCodeWithCtxTrampoline patches originPtr to set x26=ctxtPtr then branch directly to codePtr,
// and also builds a "fixed origin" trampoline (fixOriginPtr) that can be used to call the original
// function body without unpatching the origin.
//
// The trampoline contains relocated origin prologue instructions and then jumps back to origin+N.
func PtrCodeWithCtxTrampoline(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	jumpBytes, err := jmpToCodeWithCtxJump(originPtr, ctxtPtr, codePtr)
	if err != nil {
		return nil, err
	}
	fixOriginPtr, err := buildFixOriginTrampoline(originPtr, len(jumpBytes))
	if err != nil {
		return nil, err
	}
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrCodeWithCtxTrampoline origin=0x%x ctxt=0x%x code=0x%x jumpLen=%d fixOrigin=0x%x",
		originPtr, ctxtPtr, codePtr, len(jumpBytes), fixOriginPtr)
	return guard, nil
}

func appendU32LE(dst []byte, w uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], w)
	return append(dst, b[:]...)
}

func emitMovX26Imm64(dst []byte, ptr uintptr) []byte {
	d0d1 := ptr & 0xFFFF
	d2d3 := ptr >> 16 & 0xFFFF
	d4d5 := ptr >> 32 & 0xFFFF
	d6d7 := ptr >> 48 & 0xFFFF
	dst = append(dst, movImm(_0b10, 0, d0d1)...) // MOVZ x26
	dst = append(dst, movImm(_0b11, 1, d2d3)...) // MOVK x26
	dst = append(dst, movImm(_0b11, 2, d4d5)...) // MOVK x26
	dst = append(dst, movImm(_0b11, 3, d6d7)...) // MOVK x26
	return dst
}

// jmpToCodeWithCtxJump emits a unified jump sequence:
// - set x26=ctxtPtr
// - jump to codePtr using:
//   - short B imm26 when in range (smaller/faster)
//   - otherwise load absolute addr into x16 and BR x16 (stable, no deref)
func jmpToCodeWithCtxJump(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr) ([]byte, error) {
	// 16 bytes: MOVZ/MOVK x26, ctxtPtr
	res := make([]byte, 0, 40)
	res = emitMovX26Imm64(res, ctxtPtr)

	// 4 bytes: B imm26 to codePtr from the address of this instruction (origin+16).
	from := originPtr + 16
	if from%4 != 0 || codePtr%4 != 0 {
		return nil, fmt.Errorf("PtrCodeWithCtx: unaligned branch from=0x%x to=0x%x", from, codePtr)
	}
	delta := int64(codePtr) - int64(from)
	if delta%4 != 0 {
		return nil, fmt.Errorf("PtrCodeWithCtx: branch delta not aligned: %d", delta)
	}
	imm := delta >> 2
	if imm >= -(1<<25) && imm < (1<<25) {
		ins := uint32(0x14000000) | (uint32(imm) & 0x03FFFFFF) // B imm26
		res = appendU32LE(res, ins)
		return res, nil
	}

	// Long jump: MOVZ/MOVK x16, codePtr; BR x16.
	for _, w := range loadAddrToX16(codePtr) {
		res = appendU32LE(res, w)
	}
	res = appendU32LE(res, branchRegX16(false))
	return res, nil
}
