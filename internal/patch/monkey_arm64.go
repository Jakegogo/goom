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

func jmpToFunctionValue(_, double uintptr) []byte {
	//func buildJmpDirective(double uintptr) []byte {
	res := make([]byte, 0, 24)
	d0d1 := double & 0xFFFF
	d2d3 := double >> 16 & 0xFFFF
	d4d5 := double >> 32 & 0xFFFF
	d6d7 := double >> 48 & 0xFFFF

	res = append(res, movImm(_0b10, 0, d0d1)...)         // MOVZ x26, double[16:0]
	res = append(res, movImm(_0b11, 1, d2d3)...)         // MOVK x26, double[32:16]
	res = append(res, movImm(_0b11, 2, d4d5)...)         // MOVK x26, double[48:32]
	res = append(res, movImm(_0b11, 3, d6d7)...)         // MOVK x26, double[64:48]
	res = append(res, []byte{0x4A, 0x03, 0x40, 0xF9}...) // LDR x10, [x26]
	res = append(res, []byte{0x40, 0x01, 0x1F, 0xD6}...) // BR x10

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
	// MOVZ/MOVK x26, imm16 (4 parts) + BR x26.
	res := make([]byte, 0, 20)
	d0d1 := to & 0xFFFF
	d2d3 := to >> 16 & 0xFFFF
	d4d5 := to >> 32 & 0xFFFF
	d6d7 := to >> 48 & 0xFFFF
	res = append(res, movImm(_0b10, 0, d0d1)...)         // MOVZ x26, to[16:0]
	res = append(res, movImm(_0b11, 1, d2d3)...)         // MOVK x26, to[32:16]
	res = append(res, movImm(_0b11, 2, d4d5)...)         // MOVK x26, to[48:32]
	res = append(res, movImm(_0b11, 3, d6d7)...)         // MOVK x26, to[64:48]
	res = append(res, []byte{0x40, 0x03, 0x1F, 0xD6}...) // BR x26
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
