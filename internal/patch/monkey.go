package patch

import (
	"fmt"
	"reflect"

	"github.com/tencent/goom/internal/logger"
)

// Patch 将函数调用指定代理函数
// origin 原始函数
// replacement 代理函数
func Patch(origin, replacement interface{}) (*Guard, error) {
	return Trampoline(origin, replacement, nil)
}

// Trampoline 将函数调用指定代理函数
// origin 原始函数
// replacement 代理函数
// trampoline 指定跳板函数(可不指定,传 nil)
func Trampoline(origin, replacement, trampoline interface{}) (*Guard, error) {
	patch := &patch{
		origin:      origin,
		replacement: replacement,
		trampoline:  trampoline,
	}

	err := patch.patch()
	if err != nil {
		return nil, err
	}

	return patch.Guard(), nil
}

// UnsafePatch 未受类型检查的 patch
// origin 原始函数
// replacement 代理函数
func UnsafePatch(origin, replacement interface{}) (*Guard, error) {
	return UnsafePatchTrampoline(origin, replacement, nil)
}

// UnsafePatchTrampoline 未受类型检查的 patch
// origin 原始函数
// replacement 代理函数
// trampoline 指定跳板函数(可不指定,传 nil)
func UnsafePatchTrampoline(origin, replacement, trampoline interface{}) (*Guard, error) {
	patch := &patch{
		origin:           origin,
		replacement:      replacement,
		trampoline:       trampoline,
		originValue:      reflect.ValueOf(origin),
		replacementValue: reflect.ValueOf(replacement),
	}

	if err := patch.unsafePatchValue(); err != nil {
		return nil, err
	}
	return patch.Guard(), nil
}

// Ptr 直接将函数跳转的新函数
// 此方式为经过函数签名检查,可能会导致栈帧无法对其导致堆栈调用异常，因此不安全请谨慎使用
// originPtr 原始函数地址
// replacement 代理函数
func Ptr(originPtr uintptr, replacement interface{}) (*Guard, error) {
	return PtrTrampoline(originPtr, replacement, nil)
}

// PtrTrampoline 直接将函数跳转的新函数(指定跳板函数)
// 此方式为经过函数签名检查,可能会导致栈帧无法对其导致堆栈调用异常，因此不安全请谨慎使用
// originPtr 原始函数地址
// replacement 代理函数
// trampoline 跳板函数地址(可不指定,传 nil)
func PtrTrampoline(originPtr uintptr, replacement, trampoline interface{}) (*Guard, error) {
	patch := &patch{
		replacement: replacement,
		trampoline:  trampoline,

		replacementValue: reflect.ValueOf(replacement),

		originPtr: originPtr,
	}

	err := patch.unsafePatchPtr()
	if err != nil {
		return nil, err
	}
	return patch.Guard(), nil
}

// InstanceMethod replaces an instance method methodName for the type target with replacementValue
// Replacement should expect the receiver (of type target) as the first argument
func InstanceMethod(originType reflect.Type, methodName string, replacement interface{}) (*Guard, error) {
	return InstanceMethodTrampoline(originType, methodName, replacement, nil)
}

// InstanceMethodTrampoline replaces an instance method methodName for the type target with replacementValue
// Replacement should expect the receiver (of type target) as the first argument
func InstanceMethodTrampoline(originType reflect.Type, methodName string, replacement interface{},
	trampoline interface{}) (*Guard, error) {
	m, ok := originType.MethodByName(methodName)
	if !ok {
		return nil, fmt.Errorf("unknown method %s", methodName)
	}

	patch := &patch{
		replacement: replacement,
		trampoline:  trampoline,

		originValue:      m.Func,
		replacementValue: reflect.ValueOf(replacement),
	}

	err := patch.patchValue()
	if err != nil {
		return nil, err
	}
	return patch.Guard(), nil
}

// Unpatch removes any monkey patches on target
// returns whether target was patched in the first place
func Unpatch(origin interface{}) bool {
	return unpatchValue(reflect.ValueOf(origin).Pointer())
}

// UnpatchInstanceMethod removes the patch on methodName of the target
// returns whether it was patched in the first place
func UnpatchInstanceMethod(originType reflect.Type, methodName string) bool {
	m, ok := originType.MethodByName(methodName)
	if !ok {
		logger.Debugf(fmt.Sprintf("unknown method %s", methodName))
		return false
	}

	return unpatchValue(m.Func.Pointer())
}

// UnpatchAll removes all applied monkey patches
func UnpatchAll() {
	for target, p := range patches {
		p.unpatch()
		delete(patches, target)
	}
}

// unpatchValue removes a monkeypatch from the specified function
// returns whether the function was patched in the first place
func unpatchValue(origin uintptr) bool {
	p, ok := patches[origin]
	if !ok {
		return false
	}

	p.unpatch()
	delete(patches, origin)
	return true
}
