// Package bytecode 是内存字节码层面操作的工具集
package bytecode

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/tencent/goom/internal/arch/x86asm"
)

// opExpand 短地址指令 -> 长地址指令
// 原始函数内部的短地址跳转无法满足长距离跳转时候,需要修改为长地址跳转, 因此同时需要将操作符指令修改为对应的长地址操作符指令
var opExpand = map[uint32][]byte{
	0x74: {0x0F, 0x84}, // JE: 74->0F
	0x76: {0x0F, 0x86}, // JBE: 76->0F
	0x7F: {0x0F, 0x8F},
	0xEB: {0xE9}, // JMP: EB->E9 (Jump near, relative, displacement relative to next instruction.)
}

// DecodeRelativeAddr decode relative address, if jump to the front of current pos, return negative values
func DecodeRelativeAddr(ins *x86asm.Inst, block []byte, offset int) int {
	var isAdd = true
	for i := 0; i < len(ins.Args); i++ {
		arg := ins.Args[i]
		if arg == nil {
			break
		}
		addrArgs := arg.String()
		if strings.HasPrefix(addrArgs, ".-") || strings.Contains(addrArgs, "RIP-") {
			isAdd = false
		}
	}

	relativeAddr := DecodeAddress(block[offset:offset+ins.PCRel], ins.PCRel)
	if !isAdd && relativeAddr > 0 {
		relativeAddr = -relativeAddr
	}
	return relativeAddr
}

// EncodeAddress 写入地址参数到函数字节码
// len 地址值的位数
// val 地址值
// add 偏移量, 可为负数
func EncodeAddress(ops []byte, addr []byte, addrLen int, val int, add int) []byte {
	switch addrLen {
	case 1:
		if !isByteOverflow((int32)(int8(val)) + (int32)(add)) {
			addr[0] = byte((int)(int8(val)) + add)
			return toInst(ops, addr)
		}
		if opsNew, ok := opExpand[uint32(ops[0])]; ok {
			addr = make([]byte, 4)
			LittleEndian.PutInt32(addr, (int32)(int8(val))+int32(add)-
				int32(len(addr)-addrLen)-int32(len(opsNew)-len(ops))) // 新增了4个字节,需要减去
			ops = opsNew
			return toInst(ops, addr)
		}
		panic("address overflow:" + hex.EncodeToString(ops) + ", addr:" + hex.EncodeToString(addr[:addrLen]))
	case 2:
		if !isInt16Overflow((int32)(int8(val)) + (int32)(add)) {
			LittleEndian.PutInt16(addr, int16(val)+int16(add))
			return toInst(ops, addr)
		}
		if opsNew, ok := opExpand[uint32(ops[0])<<16+uint32(ops[1])]; ok {
			addr = make([]byte, 4)
			LittleEndian.PutInt32(addr, (int32)(int8(val))+int32(add)-
				int32(len(addr)-addrLen)-int32(len(opsNew)-len(ops))) // 新增了4个字节,需要减去
			ops = opsNew
			return toInst(ops, addr)
		}
		panic("address overflow:" + hex.EncodeToString(ops) + ", addr:" + hex.EncodeToString(addr[:addrLen]))
	case 4:
		LittleEndian.PutInt32(addr, int32(val)+int32(add))
		return toInst(ops, addr)
	case 8:
		LittleEndian.PutInt64(addr, int64(val)+int64(add))
		return toInst(ops, addr)
	default:
		panic(fmt.Sprintf("address overflow check error: add len not support:%d", addrLen))
	}
}

func toInst(ops, addr []byte) []byte {
	result := make([]byte, 0)
	result = append(result, ops...)
	return append(result, addr...)
}

// DecodeAddress 从函数字节码中解析地址数值
// len 地址值的位数
func DecodeAddress(bytes []byte, len int) int {
	switch len {
	case 1:
		return int(int8(bytes[0]))
	case 2:
		return int(LittleEndian.Int16(bytes))
	case 4:
		return int(LittleEndian.Int32(bytes))
	case 8:
		return int(LittleEndian.Int64(bytes))
	default:
		panic(fmt.Sprintf("decode address error: add len not support:%d", len))
	}
}

// isByteOverflow 字节是否溢出
func isByteOverflow(v int32) bool {
	if v > 0 {
		if v > math.MaxInt8 {
			return true
		}
	} else {
		if v < math.MinInt8 {
			return true
		}
	}
	return false
}

// isInt16Overflow  init16是否溢出
func isInt16Overflow(v int32) bool {
	if v > 0 {
		if v > math.MaxInt16 {
			return true
		}
	} else {
		if v < math.MinInt16 {
			return true
		}
	}
	return false
}

// nolint
func isInt32Overflow(v int64) bool {
	if v > 0 {
		if v > math.MaxInt32 {
			return true
		}
	} else {
		if v < math.MinInt32 {
			return true
		}
	}
	return false
}
