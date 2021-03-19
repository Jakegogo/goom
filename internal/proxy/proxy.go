// Packge proxy封装了给各种类型的代理(或较patch)中间层
// 负责比如外部传如类型校验、私有函数名转换成uintptr、trampoline初始化、并发proxy等
package proxy

import (
	"errors"
	"fmt"
	"reflect"

	"git.code.oa.com/goom/mocker/internal/logger"
	"git.code.oa.com/goom/mocker/internal/patch"
	"git.code.oa.com/goom/mocker/internal/unexports"
)

// StaticProxyByName 静态代理(函数或方法)
// @param genCallableFunc 函数名称
// @param proxyFunc 代理函数实现
// @param trampolineFunc 跳板函数,即代理后的原始函数定义;跳板函数的签名必须和原函数一致,值不能为空
func StaticProxyByName(funcName string, proxyFunc interface{}, trampolineFunc interface{}) (*patch.PatchGuard, error) {
	e := checkTrampolineFunc(trampolineFunc)
	if e != nil {
		return nil, e
	}

	originFuncPtr, err := unexports.FindFuncByName(funcName)
	if err != nil {
		return nil, err
	}

	logger.LogInfo("start StaticProxyByName genCallableFunc=", funcName)

	// 保证patch和Apply原子性
	patch.PatchLock()
	defer patch.PatchUnlock()

	// gomonkey添加函数hook
	patchGuard, err := patch.PatchPtrTrampoline(originFuncPtr, proxyFunc, trampolineFunc)
	if err != nil {
		logger.LogError("StaticProxyByName fail genCallableFunc=", funcName, ":", err)
		return nil, err
	}

	// 构造原先方法实例值
	logger.LogDebug("OrignUintptr is:", fmt.Sprintf("0x%x", patchGuard.OriginFunc()))
	logger.LogInfo("static proxy[trampoline] ok, genCallableFunc=", funcName)

	return patchGuard, nil
}

// StaticProxyByFunc 静态代理(函数或方法)
// @param funcDef 原始函数定义
// @param proxyFunc 代理函数实现
// @param originFunc 跳板函数即代理后的原始函数定义(值为nil时,使用公共的跳板函数, 不为nil时使用指定的跳板函数)
func StaticProxyByFunc(funcDef interface{}, proxyFunc, trampolineFunc interface{}) (*patch.PatchGuard, error) {
	e := checkTrampolineFunc(trampolineFunc)
	if e != nil {
		return nil, e
	}

	logger.LogInfo("start StaticProxyByFunc funcDef=", funcDef)

	// 保证patch和Apply原子性
	patch.PatchLock()
	defer patch.PatchUnlock()

	// gomonkey添加函数hook
	patchGuard, err := patch.PatchTrampoline(
		reflect.Indirect(reflect.ValueOf(funcDef)).Interface(), proxyFunc, trampolineFunc)
	if err != nil {
		logger.LogError("StaticProxyByFunc fail funcDef=", funcDef, ":", err)
		return nil, err
	}
	// 构造原先方法实例值
	logger.LogDebug("OrignUintptr is:", fmt.Sprintf("0x%x", patchGuard.OriginFunc()))

	if patch.IsPtr(trampolineFunc) {
		_, err = unexports.CreateFuncForCodePtr(trampolineFunc, patchGuard.OriginFunc())
		if err != nil {
			logger.LogError("StaticProxyByFunc fail funcDef=", funcDef, ":", err)
			patchGuard.Unpatch()

			return nil, err
		}
	}

	logger.LogDebug("static proxy ok funcDef=", funcDef)

	return patchGuard, nil
}

// StaticProxyByMethod 方法静态代理
// @param target 类型
// @param methodName 方法名
// @param proxyFunc 代理函数实现
// @param trampolineFunc 跳板函数即代理后的原始方法定义(值为nil时,使用公共的跳板函数, 不为nil时使用指定的跳板函数)
func StaticProxyByMethod(target reflect.Type, methodName string, proxyFunc,
	trampolineFunc interface{}) (*patch.PatchGuard, error) {
	e := checkTrampolineFunc(trampolineFunc)
	if e != nil {
		return nil, e
	}

	logger.LogInfo("start StaticProxyByMethod genCallableFunc=", target, ".", methodName)

	// 保证patch和Apply原子性
	patch.PatchLock()
	defer patch.PatchUnlock()

	// gomonkey添加函数hook
	patchGuard, err := patch.PatchInstanceMethodTrampoline(target, methodName, proxyFunc, trampolineFunc)
	if err != nil {
		logger.LogError("StaticProxyByMethod fail type=", target, "methodName=", methodName, ":", err)
		return nil, err
	}

	// 构造原先方法实例值
	logger.LogDebug("OrignUintptr is:", fmt.Sprintf("0x%x", patchGuard.OriginFunc()))

	if patch.IsPtr(trampolineFunc) {
		_, err = unexports.CreateFuncForCodePtr(trampolineFunc, patchGuard.OriginFunc())
		if err != nil {
			logger.LogError("StaticProxyByMethod fail method=", target, ".", methodName, ":", err)
			patchGuard.Unpatch()

			return nil, err
		}
	}

	logger.LogDebug("static proxy ok genCallableFunc=", target, ".", methodName)

	return patchGuard, nil
}

// checkTrampolineFunc 检测TrampolineFunc类型
func checkTrampolineFunc(trampolineFunc interface{}) error {
	if trampolineFunc != nil {
		if reflect.ValueOf(trampolineFunc).Kind() != reflect.Func &&
			reflect.ValueOf(trampolineFunc).Elem().Kind() != reflect.Func {
			return errors.New("trampolineFunc has to be a exported func")
		}
	}
	return nil
}
