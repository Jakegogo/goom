//go:build go1.17 && amd64
// +build go1.17,amd64

package argdump

import "unsafe"

const (
	// internal/abi constants for amd64 (Go 1.17+ regabi).
	intArgRegs   = 9
	floatArgRegs = 15
	floatRegSize = uintptr(8)
	ptrSize      = uintptr(unsafe.Sizeof(uintptr(0)))
)


