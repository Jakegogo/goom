// Package patch 对不同类型的函数、方法、未导出函数、进行hook
package patch

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"

	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/logger"
)

var (
	// patches 缓存
	patches = make(map[uintptr]*patch)
	// lock patches 缓存的读写锁定
	patchesLock = sync.Mutex{}
)

// lock 锁定 patches map 和内存指令读写
func lock() {
	patchesLock.Lock()
}

// unlock 解锁
func unlock() {
	patchesLock.Unlock()
}

// patch 一个可以 Apply 的 patch
type patch struct {
	origin      interface{} // 原始函数,即要mock的目标函数, 相对于代理函数来说叫原始函数
	replacement interface{} // 代理函数
	trampoline  interface{} // 跳板函数

	originValue      reflect.Value
	replacementValue reflect.Value

	// 指针管理
	originPtr      uintptr
	replacementPtr uintptr
	trampolinePtr  uintptr
	fixOriginPtr   uintptr

	originBytes []byte
	jumpBytes   []byte

	guard *Guard
}

// patchValue 对 value 进行应用代理
func (p *patch) patch() error {
	p.originValue = reflect.ValueOf(p.origin)
	p.replacementValue = reflect.ValueOf(p.replacement)
	return p.patchValue()
}

// patchValue 对 value 进行应用代理
func (p *patch) patchValue() error {
	SignatureEquals(p.originValue.Type(), p.replacementValue.Type())
	return p.unsafePatchValue()
}

// unsafePatchValue 不做类型检查
func (p *patch) unsafePatchValue() error {
	if p.originValue.Kind() != reflect.Func {
		return errors.New("target has to be a ExportFunc")
	}
	if p.replacementValue.Kind() != reflect.Func {
		return errors.New("replacementValue has to be a ExportFunc")
	}
	originPointer := p.originValue.Pointer()
	tracePatchOrigin("origin", originPointer, p.originValue.Type().String())
	p.originPtr = originPointer

	// fix for generics variants
	funcName := runtime.FuncForPC(originPointer).Name()
	if IsGenericsFunc(funcName) {
		innerPointer, err := bytecode.GetInnerFunc(64, originPointer)
		if err == nil && innerPointer != 0 {
			p.originPtr = innerPointer
		}
	}
	return p.unsafePatchPtr()
}

func tracePatchOrigin(label string, addr uintptr, typeName string) {
	if os.Getenv("GOOM_TRACE_MPROTECT") != "1" {
		return
	}
	fn := runtime.FuncForPC(addr)
	name := "<unknown>"
	if fn != nil {
		name = fn.Name()
	}
	_, _ = fmt.Fprintf(os.Stderr, "[mprotect] hookOrigin %s addr=0x%x func=%s type=%s\n", label, addr, name, typeName)
}

// unsafePatchPtr 不做类型检查
func (p *patch) unsafePatchPtr() error {
	replacementPointer := p.replacementValue.Pointer()
	p.replacementPtr = replacementPointer
	if p.trampoline != nil {
		trampolinePtr, err := bytecode.GetTrampolinePtr(p.trampoline)
		if err != nil {
			return fmt.Errorf("patch error when get ptr of trampoline: %w", err)
		}
		p.trampolinePtr = trampolinePtr
	}
	return p.replaceFunc()
}

// replaceFunc 替换函数
func (p *patch) replaceFunc() error {
	lock()
	defer unlock()

	if _, ok := patches[p.originPtr]; ok {
		unpatchValue(p.originPtr)
	}
	patches[p.originPtr] = p

	replacementInAddr := (uintptr)(bytecode.GetPtr(p.replacementValue))
	jumpData, err := genJumpData(p.originPtr, replacementInAddr, p.replacementPtr)
	if err != nil {
		if errors.Unwrap(err) == errAlreadyPatch {
			if pc, ok := patches[p.originPtr]; ok {
				bytecode.PrintInstf("origin bytes", pc.originPtr, pc.originBytes, logger.WarningLevel)
			}
		}
		return err
	}
	p.jumpBytes = jumpData

	originBytes, err := checkAndReadOriginBytes(p.originPtr, len(jumpData))
	if err != nil {
		return err
	}
	p.originBytes = originBytes

	// 是否修复指令
	if p.trampolinePtr > 0 {
		fixOriginPtr, err := fixOrigin(p.originPtr, p.trampolinePtr, len(jumpData))
		if err != nil {
			return err
		}
		p.fixOriginPtr = fixOriginPtr
	}

	return nil
}

// unpatch do unpatch by uint ptr
func (p *patch) unpatch() {
	p.Guard().Unpatch()
}

// Guard 获取 PatchGuard
func (p *patch) Guard() *Guard {
	if p.guard != nil {
		return p.guard
	}
	p.guard = &Guard{
		origin:       p.originPtr,
		originBytes:  p.originBytes,
		jumpBytes:    p.jumpBytes,
		fixOriginPtr: p.fixOriginPtr,
		applied:      false,
	}
	return p.guard
}

// registerPatchLocked registers a patch in the global patches map and returns its Guard.
// Caller must hold the global patch lock.
func registerPatchLocked(originPtr uintptr, jumpBytes []byte, fixOriginPtr uintptr) (*Guard, error) {
	if _, ok := patches[originPtr]; ok {
		unpatchValue(originPtr)
	}

	tracePatchOrigin("register", originPtr, "<unknown>")
	originBytes, err := checkAndReadOriginBytes(originPtr, len(jumpBytes))
	if err != nil {
		return nil, err
	}

	p := &patch{
		originPtr:    originPtr,
		originBytes:  originBytes,
		jumpBytes:    jumpBytes,
		fixOriginPtr: fixOriginPtr,
	}
	patches[originPtr] = p
	bytecode.PrintInst("origin >>>>> ", originPtr, bytecode.PrintShort, logger.DebugLevel)
	return p.Guard(), nil
}
