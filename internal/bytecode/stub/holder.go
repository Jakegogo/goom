// Package stub 负责管理桩函数内存管理
package stub

import (
	"errors"
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/logger"
)

// placeHolderIns 占位实例
var placeHolderIns *PlaceHolder

// errSpaceOverflow 空间使用溢出错误
var errSpaceOverflow = errors.New("placeholder space usage overflow")

// PlaceHolder 占位对象
type PlaceHolder struct {
	// count hook 次数统计
	count int
	// off 当前占位函数使用的偏移量
	off uintptr
	// min 占位函数起始位置
	min uintptr
	// max 占位函数末尾位置
	max uintptr
}

// Placeholder 汇编函数声明: 占位函数
func Placeholder()

func init() {
	offset := reflect.ValueOf(Placeholder).Pointer()
	// 兼容 go 1.17(1.17以上会对 assembler 函数进行 wrap, 需要找到其内部的调用)
	innerOffset, err := bytecode.GetInnerFunc(64, offset)
	if innerOffset > 0 && err == nil {
		offset = innerOffset
	}

	size, err := bytecode.GetFuncSize(64, offset, false)
	if err != nil {
		logger.Error("GetFuncSize error", err)
		size = 102400
	}
	placeHolderIns = &PlaceHolder{
		count: 0,
		off:   offset,
		min:   offset,
		max:   uintptr(size) + offset,
	}
	logger.Debugf("Placeholder pointer: %d %d\n", placeHolderIns.min, offset)
}

// acquireFromHolder enough executable space from holder
// nolint
func acquireFromHolder(len int) (uintptr, *[]byte, error) {
	placeholder := atomic.LoadUintptr(&placeHolderIns.off)
	if placeholder+uintptr(len) > placeHolderIns.max {
		logger.Error("placeholder space usage overflow")
		return 0, nil, errSpaceOverflow
	}

	// add up to off
	newOffset := atomic.AddUintptr(&placeHolderIns.off, uintptr(len))
	if newOffset > placeHolderIns.max {
		logger.Error("placeholder space usage overflow", placeHolderIns.count, "hook functions")
		return 0, nil, errSpaceOverflow
	}

	bytes := (*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: placeholder,
		Len:  len,
		Cap:  len,
	}))
	return placeholder, bytes, nil
}
