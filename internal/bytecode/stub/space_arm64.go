package stub

import (
	"fmt"
	"reflect"
	_ "unsafe"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

const spaceLen = 128

var iCacheHolderAddr uintptr

// ICachePaddingLeft ClearICache 左侧占位
func ICachePaddingLeft()

// ClearICache 汇编函数声明: 清理 icache 缓存
func ClearICache()

func init() {
	iCacheHolderAddr = reflect.ValueOf(ClearICache).Pointer()
	// 兼容 go 1.17(1.17以上会对 assembler 函数进行 wrap, 需要找到其内部的调用)
	innerAddr, err := bytecode.GetInnerFunc(64, iCacheHolderAddr)
	if innerAddr > 0 && err == nil {
		iCacheHolderAddr = innerAddr
	}
	offset := reflect.ValueOf(ICachePaddingLeft).Pointer()
	logger.Debugf("icache func init success: %x", offset)
}

// WriteICacheFn 写入 icache clear 函数数据
//
//go:linkname WriteICacheFn
func WriteICacheFn(data []byte) (uintptr, error) {
	s, err := acquireICacheFn()
	if err != nil {
		return 0, err
	}
	switch s.typ {
	case TypeMMap:
		if err := writeToMMap(s.Addr, s.Space, data); err != nil {
			return 0, err
		}
		return s.Addr, nil
	case TypeHolder:
		return s.Addr, memory.WriteToNoFlushNoLock(s.Addr, data)
	default:
		return 0, fmt.Errorf("ICacheFn write fail, illegal type: %d", s.typ)
	}
}

// acquireICacheFn 获取 icache 执行空间
func acquireICacheFn() (*Space, error) {
	// IMPORTANT (darwin/arm64 stability):
	// The ClearICache helper is used by write paths that may themselves be allocating/writing
	// executable memory. Allocating the ClearICache helper via mmap would introduce a bootstrap
	// problem (we'd need ClearICache to safely finalize/expose executable code).
	//
	// Therefore we always use the in-binary holder region for the ClearICache helper.
	return &Space{
		Addr:  iCacheHolderAddr,
		Space: nil,
		typ:   TypeHolder,
	}, nil
}
