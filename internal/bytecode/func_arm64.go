package bytecode

import (
	"bytes"
	"encoding/hex"

	"github.com/tencent/goom/internal/arch/arm64asm"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

// defaultLength 默认指令长度
const defaultLength = 4

// funcPrologue 函数的开头指纹,用于不同OS获取不同的默认值
var funcPrologue = armFuncPrologue64

const (
	// CallInsName call 指令名称
	CallInsName = "B"
	// CallInsName1 call 指令名称
	CallInsName1 = "BL"
)

// GetFuncSize get func binary size
// not absolutely safe
func GetFuncSize(_ int, start uintptr, minimal bool) (length int, err error) {
	funcSizeReadLock.Lock()
	defer funcSizeReadLock.Unlock()

	defer func() {
		funcSizeCache[start] = length
	}()

	if l, ok := funcSizeCache[start]; ok {
		return l, nil
	}

	prologueLen := len(funcPrologue)
	code := memory.RawRead(start, 16) // instruction takes at most 16 bytes

	int0Found := false
	curLen := 0
	for {
		inst, err := arm64asm.Decode(code)
		if err != nil {
			return curLen, nil
		}

		if inst.Op == 0 && code[0] == 0x00 {
			// 0x00 -> int0, trap to debugger, padding to function end
			if minimal {
				return curLen, nil
			}
			int0Found = true
		} else if int0Found {
			return curLen, nil
		}

		curLen += defaultLength
		code = memory.RawRead(start+uintptr(curLen), 16) // instruction takes at most 16 bytes

		if bytes.Equal(funcPrologue, code[:prologueLen]) {
			return curLen, nil
		}
	}
}

// PrintInstf 调试内存指令替换,对原指令、替换之后的指令进行输出对比
func PrintInstf(title string, from uintptr, copyOrigin []byte, level int) {
	if logger.LogLevel < level {
		return
	}
	logger.Important(title)

	startAddr := uint64(from)
	for pos := 0; pos < len(copyOrigin); {
		// read 16 bytes at most each time
		endPos := pos + 16
		if endPos > len(copyOrigin) {
			endPos = len(copyOrigin)
		}

		code := copyOrigin[pos:endPos]
		ins, err := arm64asm.Decode(code)
		if err != nil {
			logger.Importantf("[0] 0x%x: inst decode error:%s", startAddr+(uint64)(pos), err)
			pos += 4
			continue
		}

		if ins.Op == 0 {
			if code[0] == 0x00 {
				pos += 1
			} else {
				pos += 4
			}
			continue
		}

		logger.Importantf("[%d] 0x%x:\t%s\t\t%-30s\t\t%s", 4,
			startAddr+(uint64)(pos), ins.Op, ins.String(), hex.EncodeToString(code[:4]))

		pos += 4
	}
}

// GetInnerFunc Get the first real func location from wrapper
// not absolutely safe
func GetInnerFunc(mode int, start uintptr) (uintptr, error) {
	prologueLen := len(funcPrologue)
	code := memory.RawRead(start, 16) // instruction takes at most 16 bytes

	int0Found := false
	curLen := 0
	for {
		inst, err := arm64asm.Decode(code)
		if err != nil {
			return 0, err
		}

		if inst.Op == 0 && code[0] == 0x00 {
			int0Found = true
		} else if int0Found {
			return 0, nil
		}

		if inst.Op.String() == CallInsName || inst.Op.String() == CallInsName1 {
			rAddr, ok := (inst.Args[0]).(arm64asm.PCRel)
			if ok {
				if rAddr >= 0 {
					return start + uintptr(curLen) + uintptr(rAddr), nil
				}
				if curLen+int(rAddr) < 0 {
					return start + uintptr(curLen) - uintptr(-rAddr), nil
				}
			}
		}

		curLen += defaultLength
		code = memory.RawRead(start+uintptr(curLen), 16) // instruction takes at most 16 bytes

		if bytes.Equal(funcPrologue, code[:prologueLen]) {
			return 0, nil
		}
		if curLen > 4096 {
			return 0, nil
		}
	}
}
