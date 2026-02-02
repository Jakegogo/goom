//go:build go1.17 && arm64
// +build go1.17,arm64

package argdump

import "unsafe"

const (
	// internal/abi constants for arm64 (Go 1.17+ regabi).
	intArgRegs   = 16
	floatArgRegs = 16
	floatRegSize = uintptr(8)
	ptrSize      = uintptr(unsafe.Sizeof(uintptr(0)))
)


