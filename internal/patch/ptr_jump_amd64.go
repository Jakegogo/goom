//go:build amd64
// +build amd64

package patch

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	"github.com/tencent/goom/internal/logger"
)

// PtrFuncVal patches originPtr to jump to a *func value* located at funcVal.
// Unlike Ptr/Trampoline, this is specialized for func values (closures) and always
// sets the amd64 context register (DX) to funcVal before jumping.
//
// If fixOriginPtr is non-zero, it is recorded in the returned Guard for callers
// that need to call the original function.
func PtrFuncVal(originPtr uintptr, funcVal unsafe.Pointer, fixOriginPtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	// For func values, ctxt is the func value pointer, and code pointer is its first word.
	funcValPtr := uintptr(funcVal)
	codePtr := *(*uintptr)(funcVal)
	jumpBytes := jmpToCodeWithCtxJump(originPtr, funcValPtr, codePtr)
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrFuncVal origin=0x%x funcValPtr=0x%x code=0x%x jumpLen=%d", originPtr, funcValPtr, codePtr, len(jumpBytes))
	return guard, nil
}

// PtrCodeWithCtx patches originPtr to set DX=ctxtPtr then jump to codePtr.
func PtrCodeWithCtx(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr, fixOriginPtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	jumpBytes := jmpToCodeWithCtxJump(originPtr, ctxtPtr, codePtr)
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrCodeWithCtx origin=0x%x ctxt=0x%x code=0x%x jumpLen=%d", originPtr, ctxtPtr, codePtr, len(jumpBytes))
	return guard, nil
}

// PtrCodeWithCtxTrampoline is like PtrCodeWithCtx but also builds a fixed-origin trampoline
// that can be used to call the original function body without unpatching.
func PtrCodeWithCtxTrampoline(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	jumpBytes := jmpToCodeWithCtxJump(originPtr, ctxtPtr, codePtr)
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

func jmpToCodeWithCtxJump(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr) []byte {
	// movabs rdx, ctxtPtr
	// jmp rel32 codePtr (if in range) else movabs rax, codePtr; jmp rax
	res := make([]byte, 0, 32)
	res = append(res, 0x48, 0xBA) // MOV RDX, imm64
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(ctxtPtr))
	res = append(res, b[:]...)

	// Attempt short JMP rel32 from after the instruction.
	from := originPtr + uintptr(len(res)) + 5 // jmp rel32 is 5 bytes; RIP after insn
	delta := int64(codePtr) - int64(from)
	if delta >= -(1<<31) && delta <= (1<<31-1) {
		res = append(res, 0xE9) // JMP rel32
		var d [4]byte
		binary.LittleEndian.PutUint32(d[:], uint32(int32(delta)))
		res = append(res, d[:]...)
		return res
	}

	// Long jump: movabs rax, codePtr; jmp rax
	res = append(res, 0x48, 0xB8) // MOV RAX, imm64
	binary.LittleEndian.PutUint64(b[:], uint64(codePtr))
	res = append(res, b[:]...)
	res = append(res, 0xFF, 0xE0) // JMP RAX
	return res
}

// Sanity check for ptr size assumptions.
func init() {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		panic(fmt.Sprintf("amd64 requires ptrSize=8, got %d", unsafe.Sizeof(uintptr(0))))
	}
}


