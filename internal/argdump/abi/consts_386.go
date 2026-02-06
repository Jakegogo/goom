//go:build go1.17 && 386
// +build go1.17,386

package abi

import "unsafe"

const (
	// IntArgRegs is the number of integer argument registers for 386 (Go 1.17+ regabi).
	IntArgRegs = 0
	// FloatArgRegs is the number of floating-point argument registers for 386.
	FloatArgRegs = 0
	// FloatRegSize is the size of a floating-point register in bytes.
	FloatRegSize = uintptr(0)
	// PtrSize is the size of a pointer in bytes.
	PtrSize = uintptr(unsafe.Sizeof(uintptr(0)))
)
