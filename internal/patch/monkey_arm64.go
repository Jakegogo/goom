package patch

import (
	"encoding/binary"
	"unsafe"
)

const (
	_0b1      = 1  // _0b1
	_0b10     = 2  // 0b10
	_0b11     = 3  // 0b11
	_0b100101 = 37 // 0b100101
)

// nopOpcode 空指令插入到原函数开头第一个字节, 用于判断原函数是否已经被Patch过
var nopOpcode = []byte{0xD5, 0x03, 0x20, 0x1F}

func jmpToFunctionValue(from, to uintptr) []byte {
	// Prefer a short PC-relative literal load when possible:
	//   LDR X10, [PC, #imm]  ; load *(to) (the code pointer inside the func value)
	//   BR  X10
	// LDR (literal) range: imm19<<2 => +/- 1MB.
	if from%4 == 0 && to%4 == 0 {
		delta := int64(to) - int64(from+4) // PC is the address of this instruction + 4
		if delta%4 == 0 {
			imm19 := delta >> 2
			if imm19 >= -(1<<18) && imm19 < (1<<18) {
				ins := uint32(0x58000000) | (uint32(imm19)&0x7FFFF)<<5 | 10 // LDR X10, #imm
				out := make([]byte, 8)
				binary.LittleEndian.PutUint32(out[0:4], ins)
				copy(out[4:8], []byte{0x40, 0x01, 0x1F, 0xD6}) // BR x10
				return out
			}
		}
	}

	// Long jump path must NOT clobber x26.
	// x26 is callee-saved and is used by reflect.makeFuncStub as the context register.
	// Clobbering it causes ctxt corruption and can crash in callDump/reflect.callReflect.
	//
	// Use x16/x17 (IP0/IP1) which are scratch registers:
	//   MOVZ/MOVK x16, to
	//   LDR x17, [x16]   ; load *(to) (the code pointer inside the func value)
	//   BR  x17
	res := make([]byte, 0, 24)
	for _, w := range loadAddrToX16(to) {
		var ins [4]byte
		binary.LittleEndian.PutUint32(ins[:], w)
		res = append(res, ins[:]...)
	}
	res = append(res, []byte{0x11, 0x02, 0x40, 0xF9}...) // LDR x17, [x16]
	res = append(res, []byte{0x20, 0x02, 0x1F, 0xD6}...) // BR x17
	return res
}

func movImm(opc, shift int, val uintptr) []byte {
	var m uint32 = 26          // rd
	m |= uint32(val) << 5      // imm16
	m |= uint32(shift&3) << 21 // hw
	m |= _0b100101 << 23       // const
	m |= uint32(opc&0x3) << 29 // opc
	m |= _0b1 << 31            // sf

	res := make([]byte, 4)
	*(*uint32)(unsafe.Pointer(&res[0])) = m

	return res
}

// jmpToOriginFunctionValue Assembles a jump to a function value
func jmpToOriginFunctionValue(from, to uintptr) (value []byte) {
	// Prefer a short relative branch when possible (smaller & faster).
	// B imm26 range: +/- 128MB.
	if from%4 == 0 && to%4 == 0 {
		delta := int64(to) - int64(from)
		imm := delta >> 2
		if delta%4 == 0 && imm >= -(1<<25) && imm < (1<<25) {
			ins := uint32(0x14000000) | (uint32(imm) & 0x03FFFFFF) // B imm26
			out := make([]byte, 4)
			binary.LittleEndian.PutUint32(out, ins)
			return out
		}
	}

	// Fallback: absolute branch via register (no deref; direct jump to code addr).
	// Must NOT clobber x26; use x16 as scratch.
	res := make([]byte, 0, 20)
	for _, w := range loadAddrToX16(to) {
		var ins [4]byte
		binary.LittleEndian.PutUint32(ins[:], w)
		res = append(res, ins[:]...)
	}
	{
		var ins [4]byte
		binary.LittleEndian.PutUint32(ins[:], branchRegX16(false))
		res = append(res, ins[:]...)
	}
	return res
}

// checkAlreadyPatch 检测是否已经patch
func checkAlreadyPatch(origin []byte) bool {
	for i := 0; i < len(nopOpcode); i++ {
		if origin[i] != nopOpcode[i] {
			return false
		}
	}
	return true
}
