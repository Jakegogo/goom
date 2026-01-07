package patch

import (
	"encoding/binary"
	"unsafe"
)

// nopOpcode 空指令插入到原函数开头第一个字节, 用于判断原函数是否已经被 Patch 过
const nopOpcode byte = 0x90

// jmpToFunctionValue Assembles a jump to a function value
func jmpToFunctionValue(from, to uintptr) (value []byte) {
	// Prefer a short RIP-relative indirect jump when the func value pointer is close enough.
	// Encoding: NOP; JMP QWORD PTR [RIP+disp32]
	//   90
	//   FF 25 xx xx xx xx
	// This keeps the leading NOP marker so checkAlreadyPatch keeps working.
	const (
		nopLen = 1
		jmpLen = 6
	)
	base := int64(from + nopLen + jmpLen) // RIP after the JMP instruction
	disp := int64(to) - base
	const (
		minInt32 = int64(-1 << 31)
		maxInt32 = int64((1 << 31) - 1)
	)
	if disp >= minInt32 && disp <= maxInt32 {
		out := make([]byte, 0, nopLen+jmpLen)
		out = append(out, nopOpcode, 0xFF, 0x25)
		imm := make([]byte, 4)
		binary.LittleEndian.PutUint32(imm, uint32(int32(disp)))
		out = append(out, imm...)
		return out
	}

	return []byte{
		0x90, // NOP
		0x48, 0xBA,
		byte(to),
		byte(to >> 8),
		byte(to >> 16),
		byte(to >> 24),
		byte(to >> 32),
		byte(to >> 40),
		byte(to >> 48),
		byte(to >> 56), // movabs rdx,to
		0xFF, 0x22,     // jmp QWORD PTR [rdx]
	}
}

// jmpToOriginFunctionValue Assembles a jump to a function value
func jmpToOriginFunctionValue(from, to uintptr) (value []byte) {
	if relative(from, to) {
		var dis uint32
		if to > from {
			dis = uint32(int32(to-from) - 5)
		} else {
			dis = uint32(-int32(from-to) - 5)
		}

		return []byte{
			0xe9,
			byte(dis),
			byte(dis >> 8),
			byte(dis >> 16),
			byte(dis >> 24),
		}
	}

	return []byte{
		0x48, 0xBA,
		byte(to),
		byte(to >> 8),
		byte(to >> 16),
		byte(to >> 24),
		byte(to >> 32),
		byte(to >> 40),
		byte(to >> 48),
		byte(to >> 56), // movabs rdx,to
		0xFF, 0x22,     // jmp QWORD PTR [rdx]
	}
}

// relative 判断两个指针间隔是否可以用相对地址表示
func relative(from uintptr, to uintptr) bool {
	delta := int64(from - to)
	if unsafe.Sizeof(uintptr(0)) == unsafe.Sizeof(int32(0)) {
		delta = int64(int32(from - to))
	}

	// 跨度大于2G 时
	relative := delta <= 0x7fffffff

	if delta < 0 {
		delta = -delta
		relative = delta <= 0x80000000
	}
	return relative
}

// checkAlreadyPatch 检测是否已经 patch
func checkAlreadyPatch(origin []byte) bool {
	return origin[0] == nopOpcode
}
