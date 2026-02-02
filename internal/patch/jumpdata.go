package patch

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

const (
	// 默认需要修复的函数长度
	defaultFuncSize = 1024
	// 默认系统位数、暂时不支持32位的
	defaultArchMod = 64
)

// errAlreadyPatch 已经 patch 过了错误
var errAlreadyPatch = errors.New("already patched")

// genJumpData 在函数 from 里面, 织入对 to 的调用指令，同时将 from 织入前的指令恢复至 trampoline 这个地址
// origin 原来的函数地址
// replacementInAddr 要跳转到的函数调用地址
// replacementCode 要跳转到的函数地址, 与 replacementInAddr 的区别详细可以参考:
// https://docs.google.com/document/d/1bMwCey-gmqZVTpRax-ESeVuZGmjwbocYs1iHplK-cjo/pub
func genJumpData(origin, replacementInAddr, replacementCode uintptr) (jumpData []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			logger.Errorf("genJumpData origin=%d replacementInAddr=%d error:%s", origin, replacementInAddr, e)
			logger.Error(string(debug.Stack()))
			if e1, ok := e.(error); ok {
				err = e1
			} else {
				err = fmt.Errorf("%s", e)
			}
		}
	}()

	logger.Infof("starting genJumpData func origin=0x%x replacementInAddr=0x%x replacementCode=0x%x ...",
		origin, replacementInAddr, replacementCode)
	bytecode.PrintInst("show replacementCode inst >>>>> ", replacementCode, 30, logger.DebugLevel)

	// 获取原函数总长度
	funcSize, e := bytecode.GetFuncSize(defaultArchMod, origin, false)
	if e != nil {
		logger.Warningf("GetFuncSize error: %v", e)
		funcSize = defaultFuncSize
	}

	// 构造跳转到代理函数的指令
	jumpData = jmpToFunctionValue(origin, replacementInAddr, replacementCode)
	// 如果需要织入的跳转指令的长度大于原函数指令长度,则任务是无法织入指令
	if len(jumpData) >= funcSize {
		bytecode.PrintInst("origin inst > ", origin, bytecode.PrintShort, logger.InfoLevel)
		return nil, fmt.Errorf(
			"jumpInstSize[%d] is bigger than origin FuncSize[%d], cannot do pathes", len(jumpData), funcSize)
	}
	return jumpData, nil
}

// checkAndReadOriginBytes 检查原函数是否已经 patch 过, 并且发挥原函数的字节码数组
func checkAndReadOriginBytes(origin uintptr, jumpDataLen int) ([]byte, error) {
	// 读取原始指令
	result := memory.RawRead(origin, jumpDataLen)
	// 判断是否已经被 patch 过
	if checkAlreadyPatch(result) {
		return nil, fmt.Errorf("origin: 0x%x is already patched, %w", origin, errAlreadyPatch)
	}
	bytecode.PrintInst("origin >>>>> ", origin, bytecode.PrintShort, logger.DebugLevel)
	return result, nil
}
