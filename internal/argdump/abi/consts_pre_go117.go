//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package abi

import "unsafe"

const (
	// PtrSize is the size of a pointer in bytes.
	PtrSize = uintptr(unsafe.Sizeof(uintptr(0)))
)
