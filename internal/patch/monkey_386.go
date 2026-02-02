package patch

// nopOpcode 空指令插入到原函数开头第一个字节, 用于判断原函数是否已经被 Patch 过
const nopOpcode = 0x90

// jmpToFunctionValue Assembles a jump to a function value
func jmpToFunctionValue(_, to uintptr, _ uintptr) []byte {
	return []byte{
		0xBA,
		byte(to),
		byte(to >> 8),
		byte(to >> 16),
		byte(to >> 24), // mov edx,to
		0xFF, 0x22,     // jmp DWORD PTR [edx]
	}
}

// checkAlreadyPatch 检测是否已经 patch
func checkAlreadyPatch(origin []byte) bool {
	if origin[0] == nopOpcode {
		return true
	}
	return false
}
