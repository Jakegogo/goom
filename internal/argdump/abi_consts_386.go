//go:build go1.17 && 386
// +build go1.17,386

package argdump

import "unsafe"

const (
	// 386 uses ABI0 (stack-based). There is no register argument area.
	intArgRegs   = 0
	floatArgRegs = 0
	floatRegSize = uintptr(0)
	ptrSize      = uintptr(unsafe.Sizeof(uintptr(0)))
)
