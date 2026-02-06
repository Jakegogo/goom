//go:build go1.17 && amd64
// +build go1.17,amd64

package abi

import "unsafe"

const (
	// IntArgRegs is the number of integer argument registers for amd64 (Go 1.17+ regabi).
	IntArgRegs = 9
	// FloatArgRegs is the number of floating-point argument registers for amd64.
	FloatArgRegs = 15
	// FloatRegSize is the size of a floating-point register in bytes.
	FloatRegSize = uintptr(8)
	// PtrSize is the size of a pointer in bytes.
	PtrSize = uintptr(unsafe.Sizeof(uintptr(0)))
)
