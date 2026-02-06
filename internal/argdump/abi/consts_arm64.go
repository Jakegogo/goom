//go:build go1.17 && arm64
// +build go1.17,arm64

package abi

import "unsafe"

const (
	// IntArgRegs is the number of integer argument registers for arm64 (Go 1.17+ regabi).
	IntArgRegs = 16
	// FloatArgRegs is the number of floating-point argument registers for arm64.
	FloatArgRegs = 16
	// FloatRegSize is the size of a floating-point register in bytes.
	FloatRegSize = uintptr(8)
	// PtrSize is the size of a pointer in bytes.
	PtrSize = uintptr(unsafe.Sizeof(uintptr(0)))
)
