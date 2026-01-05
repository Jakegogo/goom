package patch

import (
	"errors"
	"fmt"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

// fixOriginFuncToTrampoline 将原始函数 origin 的指令拷贝到 trampoline 指向的地址(在 PlaceHolder 区内存区段内)
// 对于 trampoline 模式的使用场景，本方法实现了指令移动后的修复
// 此方式不需要修正 pcvalue, 因此相对较安全
// 因 trampoline 函数需要指定签名,因此只能用于静态代理
// origin 原始函数位置
// trampoline 自定义占位函数位置(注意, 自定义占位函数一定要和原函数相同的函数签名,否则栈帧不一致会导致计算调用堆栈时候抛异常)
// jumpInstSize 跳转指令长度, 用于判断需要修复的最小指令长度
// return 跳板函数(即原函数调用入口指针)
func fixOriginFuncToTrampoline(origin uintptr, trampoline uintptr, jumpInstSize int) (uintptr, error) {
	// get origin func size
	originFuncSize, err := bytecode.GetFuncSize(defaultArchMod, origin, false)
	if err != nil {
		logger.Error("GetFuncSize error", err)
		originFuncSize = defaultFuncSize
	}

	// get trampoline func size
	trampFuncSize, err := bytecode.GetFuncSize(defaultArchMod, trampoline, false)
	if err != nil {
		logger.Error("GetFuncSize error", err)
		trampFuncSize = 24
	}
	logger.Debug("origin func size is", originFuncSize)

	// 如果需要修复的指令长度大于 trampoline 函数指令长度,则任务是无法修复
	if jumpInstSize >= trampFuncSize {
		bytecode.PrintInst("origin inst > ", origin, bytecode.PrintShort, logger.InfoLevel)
		return 0, fmt.Errorf(
			"jumpInstSize[%d] is bigger than trampoline FuncSize[%d], "+
				"please fill your trampoline func code", jumpInstSize, originFuncSize)
	}

	// copy origin function
	fixOriginData := memory.RawRead(origin, originFuncSize)
	bytecode.PrintInstf("origin inst >>>>> ", origin,
		fixOriginData[:bytecode.MinSize(bytecode.PrintMiddle, fixOriginData)], logger.DebugLevel)

	// fix relative address to placeholder
	fixedData, fixedDataSize, err := fixRelativeAddr(origin, fixOriginData, trampoline, originFuncSize, jumpInstSize)
	if err != nil {
		return 0, err
	}

	if len(fixedData) < len(fixOriginData) {
		// 追加跳转到原函数指令到修复后指令的末尾
		// append jump back to origin func position where next to the broken instructions
		jumpBackData := jmpToOriginFunctionValue(
			trampoline+uintptr(len(fixedData)),
			origin+(uintptr(fixedDataSize)))
		fixOriginData = append(fixedData, jumpBackData...)
	}

	// get trampoline func size
	trampolineFuncSize, err := bytecode.GetFuncSize(defaultArchMod, trampoline, false)
	if err != nil {
		logger.Error("Get trampoline FuncSize error", err)
		return 0, errors.New("Get trampoline FuncSize error:" + err.Error())
	}
	logger.Debug("trampoline func size is", trampolineFuncSize)

	if len(fixOriginData) > trampolineFuncSize {
		logger.Errorf("fixOriginSize[%d] is bigger than trampoline FuncSize[%d], please add your "+
			"trampoline func code", len(fixOriginData), trampolineFuncSize)
		bytecode.PrintInst("trampoline inst > ", trampoline, bytecode.PrintLong, logger.InfoLevel)

		return 0, fmt.Errorf("fixOriginSize[%d] is bigger than trampoline FuncSize[%d], "+
			"please add your trampoline func code", len(fixOriginData), trampolineFuncSize)
	}
	bytecode.PrintInst("trampoline inst > ", trampoline, bytecode.PrintLong, logger.DebugLevel)
	bytecode.PrintInstf("fixed inst >>>>> ", trampoline, fixOriginData, logger.DebugLevel)

	if err := memory.WriteTo(trampoline, fixOriginData); err != nil {
		return 0, err
	}

	bytecode.PrintInst(fmt.Sprintf("trampline copy to 0x%x", trampoline),
		trampoline, bytecode.PrintMiddle, logger.DebugLevel)
	logger.Debugf("copy to trampoline %x ", trampoline)
	return trampoline, nil
}
