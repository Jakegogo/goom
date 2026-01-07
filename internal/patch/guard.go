// Package patch 生成指令跳转(到代理函数)并替换.text 区内存
package patch

import (
	"fmt"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/logger"
)

// Guard 代理执行控制句柄, 可通过此对象进行代理还原
type Guard struct {
	origin       uintptr // 被 patch 的函数
	originBytes  []byte  // 原始字节码
	jumpBytes    []byte  // 跳转指令字节
	fixOriginPtr uintptr // 修复的函数指针
	applied      bool    // 是否已经被应用
}

// Apply 执行
func (g *Guard) Apply() {
	lock()
	defer unlock()

	g.applied = true
	// 执行函数调用地址替换(延迟执行)
	if err := memory.WriteTo(g.origin, g.jumpBytes); err != nil {
		logger.Errorf("Apply to 0x%x error: %s", g.origin, err)
	}
	bytecode.PrintInst(fmt.Sprintf("apply copy to 0x%x", g.origin), g.origin, 30, logger.DebugLevel)
}

// Unpatch 取消代理,还原指令码
// 外部调用请使用 PatchGuard.UnpatchWithLock()
func (g *Guard) Unpatch() {
	if g != nil && g.applied {
		if err := memory.WriteTo(g.origin, g.originBytes); err != nil {
			logger.Errorf("Unpatch to 0x%x error: %s", g.origin, err)
		}
		bytecode.PrintInst(fmt.Sprintf("unpatch copy to 0x%x", g.origin), g.origin, 20, logger.DebugLevel)
	}
}

// UnpatchWithLock 外部调用需要加锁
func (g *Guard) UnpatchWithLock() {
	lock()
	defer unlock()
	g.Unpatch()
}

// Restore 重新应用代理
func (g *Guard) Restore() {
	lock()
	defer unlock()
	if g != nil && g.applied {
		if err := memory.WriteTo(g.origin, g.jumpBytes); err != nil {
			logger.Errorf("Restore to 0x%x error: %s", g.origin, err)
		}
		bytecode.PrintInst(fmt.Sprintf("unpatch copy to 0x%x", g.origin), g.origin, 20, logger.DebugLevel)
	}
}

// FixOriginFunc 获取应用代理后的原函数地址(和代理前的原函数地址不一样)
func (g *Guard) FixOriginFunc() uintptr {
	return g.fixOriginPtr
}

// OriginFunc returns the address that this guard patched (the function entry).
func (g *Guard) OriginFunc() uintptr {
	return g.origin
}
