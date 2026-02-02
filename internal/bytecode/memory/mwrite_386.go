//go:build !windows && 386
// +build !windows,386

package memory

import "syscall"

// WriteTo is a minimal implementation for 32-bit x86 (386) on non-windows systems.
// It temporarily makes pages RWX, copies bytes, then restores RX.
func WriteTo(addr uintptr, data []byte) error {
	memoryAccessLock.Lock()
	defer memoryAccessLock.Unlock()

	f := RawAccess(addr, len(data))
	if err := mProtectCrossPage(addr, len(data), syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC); err != nil {
		return err
	}
	copy(f, data)
	if err := mProtectCrossPage(addr, len(data), syscall.PROT_READ|syscall.PROT_EXEC); err != nil {
		return err
	}
	return nil
}


