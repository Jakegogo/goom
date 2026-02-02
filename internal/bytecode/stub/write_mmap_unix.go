//go:build !windows
// +build !windows

package stub

import (
	"syscall"

	"github.com/tencent/goom/internal/bytecode/memory"
)

// writeToMMap writes code bytes into an mmap-backed executable region while respecting W^X:
// - temporarily make the page RW
// - copy the bytes
// - switch back to RX
// - clear instruction cache
//
// This is especially important on darwin/arm64 where RWX mappings are often disallowed.
func writeToMMap(addr uintptr, space *[]byte, data []byte) error {
	if err := syscall.Mprotect(*space, syscall.PROT_READ|syscall.PROT_WRITE); err != nil {
		return err
	}
	copy(*space, data)
	if err := syscall.Mprotect(*space, syscall.PROT_READ|syscall.PROT_EXEC); err != nil {
		return err
	}
	memory.ClearICache(addr)
	return nil
}

