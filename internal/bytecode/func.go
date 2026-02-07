package bytecode

import (
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

// 调试日志相关
const (
	// PrintShort 默认打印的指令数量(短)
	PrintShort = 20
	// PrintMiddle 默认打印的指令数量(中)
	PrintMiddle = 30
	// PrintLong 默认打印的指令数量(长)
	PrintLong = 35
)

var (
	// funcSizeCache 函数长度缓存
	funcSizeCache = make(map[uintptr]int)
	// funcSizeReadLock 并发读写 funcSizeCache 锁
	funcSizeReadLock sync.Mutex
)

var (
	//nolint it is for arm arch
	// armFuncPrologue64 arm64 func prologue
	armFuncPrologue64 = []byte{0x81, 0x0B, 0x40, 0xF9, 0xE2, 0x03, 0x00, 0x91, 0x5F, 0x00, 0x01, 0xEB}
)

// value value keep async with reflect.Value
type value struct {
	_   uintptr
	ptr unsafe.Pointer
}

// GetPtr 获取函数的调用地址(和函数的指令地址不一样)
func GetPtr(v reflect.Value) unsafe.Pointer {
	return (*value)(unsafe.Pointer(&v)).ptr
}

// isNil 判断 interface{}是否为空
func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).Elem().IsNil()
	}
	return reflect.ValueOf(i).IsNil()
}

// GetTrampolinePtr 获取跳板函数的地址
func GetTrampolinePtr(trampoline interface{}) (uintptr, error) {
	if isNil(trampoline) {
		return 0, nil
	}

	var result uintptr
	typ := reflect.TypeOf(trampoline)
	if typ.Kind() == reflect.Ptr {
		result = reflect.ValueOf(trampoline).Elem().Pointer()
	} else if typ.Kind() == reflect.Func {
		result = reflect.ValueOf(trampoline).Pointer()
	}
	logger.Debugf("trampoline value: 0x%x 0x%x", GetPtr(reflect.ValueOf(trampoline)), result)
	return result, nil
}

// IsValidPtr 判断函数 value 是否为指针类型
func IsValidPtr(value interface{}) bool {
	if value == nil {
		return false
	}
	t := reflect.TypeOf(value)
	return t.Kind() == reflect.Ptr
}

// PrintInst PrintInst 调试内存指令替换,对原指令、替换之后的指令进行输出对比
func PrintInst(name string, from uintptr, size, level int) {
	if logger.LogLevel < level {
		return
	}
	funcName := runtime.FuncForPC(from).Name()
	instBytes := memory.RawRead(from, size)
	PrintInstf(fmt.Sprintf("show [%s = %s] inst>>: ", name, funcName), from, instBytes, level)
}

// MinSize 最小 size，不超出 fixOrigin 长度的 size 大小
func MinSize(showSize int, fixOrigin []byte) int {
	if showSize > len(fixOrigin) {
		showSize = len(fixOrigin)
	}
	return showSize
}
