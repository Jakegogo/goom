//go:build 386
// +build 386

package patch

import "errors"

// 386 port does not support relocating origin instructions into a trampoline holder yet.
// We provide this symbol so the package compiles; callers should avoid trampoline mode.
func fixOriginFuncToTrampoline(origin uintptr, trampoline uintptr, jumpInstSize int) (uintptr, error) {
	_ = origin
	_ = trampoline
	_ = jumpInstSize
	return 0, errors.New("fixOriginFuncToTrampoline: unsupported on 386")
}


