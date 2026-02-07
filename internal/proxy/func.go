// Package proxy 封装了给各种类型的代理(或较 patch)中间层
// 负责比如外部传如类型校验、私有函数名转换成 uintptr、trampoline 初始化、并发 proxy 等
package proxy

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/logger"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// Func 通过函数生成代理函数
// @param funcDef 原始函数定义
// @param proxyFunc 代理函数实现
// @param originFunc 跳板函数即代理后的原始函数定义(值为 nil 时,使用公共的跳板函数, 不为 nil 时使用指定的跳板函数)
func Func(funcDef, proxyFunc, trampolineFunc interface{}) (*patch.Guard, error) {
	if e := checkTrampolineFunc(trampolineFunc); e != nil {
		return nil, e
	}

	logger.Info("start func proxy funcDef=", funcDef)
	// 添加函数 hook
	patchGuard, err := patch.Trampoline(
		reflect.Indirect(reflect.ValueOf(funcDef)).Interface(), proxyFunc, trampolineFunc)
	if err != nil {
		logger.Error("func proxy fail funcDef=", funcDef, ":", err)
		return nil, err
	}

	// 构造原先方法实例值
	logger.Debug("origin ptr is:", fmt.Sprintf("0x%x", patchGuard.FixOriginFunc()))
	if bytecode.IsValidPtr(trampolineFunc) {
		_, err = unexports2.CreateFuncForCodePtr(trampolineFunc, patchGuard.FixOriginFunc())
		if err != nil {
			logger.Error("func proxy fail funcDef=", funcDef, ":", err)
			patchGuard.Unpatch()
			return nil, err
		}
	}

	logger.Debug("func proxy ok funcDef=", funcDef)
	return patchGuard, nil
}

// FuncName 通过函数名生成代理函数
// @param genCallableMethod 函数名称
// @param proxyFunc 代理函数实现
// @param trampolineFunc 跳板函数,即代理后的原始函数定义;跳板函数的签名必须和原函数一致,值不能为空
func FuncName(funcName string, proxyFunc, trampolineFunc interface{}) (*patch.Guard, error) {
	if e := checkTrampolineFunc(trampolineFunc); e != nil {
		return nil, e
	}
	originFuncPtr, err := unexports2.FindFuncByName(funcName)
	if err != nil {
		return nil, err
	}

	logger.Info("start funcName proxy genCallableMethod=", funcName)
	// 添加函数 hook
	patchGuard, err := patch.PtrTrampoline(originFuncPtr, proxyFunc, trampolineFunc)
	if err != nil {
		logger.Error("funcName proxy fail genCallableMethod=", funcName, ":", err)
		return nil, err
	}

	// 构造原先方法实例值
	logger.Debug("origin ptr is:", fmt.Sprintf("0x%x", patchGuard.FixOriginFunc()))
	logger.Info("funcName proxy[trampoline] ok, genCallableMethod=", funcName)
	return patchGuard, nil
}

// Method 通过方法生成代理方法
// @param target 类型
// @param methodName 方法名
// @param proxyFunc 代理函数实现
// @param trampolineFunc 跳板函数即代理后的原始方法定义(值为 nil 时,使用公共的跳板函数, 不为 nil 时使用指定的跳板函数)
func Method(target reflect.Type, methodName string, proxyFunc,
	trampolineFunc interface{}) (*patch.Guard, error) {
	if e := checkTrampolineFunc(trampolineFunc); e != nil {
		return nil, e
	}

	logger.Info("start method proxy genCallableMethod=", target, ".", methodName)
	// 添加函数 hook
	patchGuard, err := patch.InstanceMethodTrampoline(target, methodName, proxyFunc, trampolineFunc)
	if err != nil {
		logger.Error("method proxy fail type=", target, "methodName=", methodName, ":", err)
		return nil, err
	}

	// 构造原先方法实例值
	logger.Debug("origin ptr is:", fmt.Sprintf("0x%x", patchGuard.FixOriginFunc()))
	if bytecode.IsValidPtr(trampolineFunc) {
		_, err = unexports2.CreateFuncForCodePtr(trampolineFunc, patchGuard.FixOriginFunc())
		if err != nil {
			logger.Error("method proxy fail method=", target, ".", methodName, ":", err)
			patchGuard.Unpatch()
			return nil, err
		}
	}

	logger.Debug("method proxy ok genCallableMethod=", target, ".", methodName)
	return patchGuard, nil
}

// checkTrampolineFunc 检测 TrampolineFunc 类型
func checkTrampolineFunc(trampolineFunc interface{}) error {
	if trampolineFunc != nil {
		if reflect.ValueOf(trampolineFunc).Kind() != reflect.Func &&
			reflect.ValueOf(trampolineFunc).Elem().Kind() != reflect.Func {
			return errors.New("trampoline func must be a exported func")
		}
	}
	return nil
}
