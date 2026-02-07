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

func jmpToFunctionValue(from, to, replacementCode uintptr) []byte {
	_ = from
	// WHY (darwin arm64e / PAC):
	// - On arm64e, funcval.fn may carry a PAC-signed pointer. A plain BR xN does NOT
	//   authenticate it, so jumping via `LDR x16, [x26]; BR x16` can execute a
	//   signature-polluted address and SIGBUS (observed on go1.17).
	// - Using the code pointer (replacementCode) avoids PAC issues because it is
	//   a direct, unsigned entry PC.
	//
	// Affected scope:
	// - go1.17+ on darwin/arm64e when patching via funcval-based trampolines.
	// - Non-arm64e (linux arm64, darwin arm64 w/o PAC) is unaffected, but this path
	//   remains safe and compatible across versions.
	//
	// Always use an absolute jump sequence that:
	// - sets x26=funcval address (ctxt register)
	// - branches to the replacement code pointer directly
	//
	// This avoids dereferencing funcval.fn on arm64e, where it may be PAC-signed
	// and not valid to execute with a plain BR.
	res := make([]byte, 0, 40)
	for _, w := range loadAddrToX26(to) {
		var ins [4]byte
		binary.LittleEndian.PutUint32(ins[:], w)
		res = append(res, ins[:]...)
	}
	code := replacementCode
	if code == 0 {
		// Fallback to old behavior if the code pointer is missing.
		res = append(res, []byte{0x50, 0x03, 0x40, 0xF9}...) // LDR x16, [x26]
		res = append(res, []byte{0x00, 0x02, 0x1F, 0xD6}...) // BR x16
		return res
	}
	for _, w := range loadAddrToX16(code) {
		var ins [4]byte
		binary.LittleEndian.PutUint32(ins[:], w)
		res = append(res, ins[:]...)
	}
	res = append(res, []byte{0x00, 0x02, 0x1F, 0xD6}...) // BR x16
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
