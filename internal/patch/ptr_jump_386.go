//go:build 386
// +build 386

package patch

import (
	"encoding/binary"
	"errors"
	"unsafe"

	"github.com/tencent/goom/internal/logger"
)

// PtrFuncVal patches originPtr to jump to a *func value* located at funcVal.
// On 386, the context register is DX.
func PtrFuncVal(originPtr uintptr, funcVal unsafe.Pointer, fixOriginPtr uintptr) (*Guard, error) {
	lock()
	defer unlock()

	funcValPtr := uintptr(funcVal)
	codePtr := *(*uintptr)(funcVal)
	jumpBytes := jmpToCodeWithCtxJump(funcValPtr, codePtr)
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

	jumpBytes := jmpToCodeWithCtxJump(ctxtPtr, codePtr)
	guard, err := registerPatchLocked(originPtr, jumpBytes, fixOriginPtr)
	if err != nil {
		return nil, err
	}
	logger.Debugf("PtrCodeWithCtx origin=0x%x ctxt=0x%x code=0x%x jumpLen=%d", originPtr, ctxtPtr, codePtr, len(jumpBytes))
	return guard, nil
}

// PtrCodeWithCtxTrampoline is not supported on 386 yet (no in-binary stub holder).
func PtrCodeWithCtxTrampoline(originPtr uintptr, ctxtPtr uintptr, codePtr uintptr) (*Guard, error) {
	return nil, errors.New("PtrCodeWithCtxTrampoline: unsupported on 386")
}

func jmpToCodeWithCtxJump(ctxtPtr uintptr, codePtr uintptr) []byte {
	// mov edx, imm32
	// mov eax, imm32
	// jmp eax
	res := make([]byte, 0, 16)

	res = append(res, 0xBA) // MOV EDX, imm32
	var d [4]byte
	binary.LittleEndian.PutUint32(d[:], uint32(ctxtPtr))
	res = append(res, d[:]...)

	res = append(res, 0xB8) // MOV EAX, imm32
	binary.LittleEndian.PutUint32(d[:], uint32(codePtr))
	res = append(res, d[:]...)

	res = append(res, 0xFF, 0xE0) // JMP EAX
	return res
}


