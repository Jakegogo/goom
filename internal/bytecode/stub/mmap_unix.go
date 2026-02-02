//go:build !windows
// +build !windows

package stub

import (
	"syscall"
	"unsafe"

	"github.com/tencent/goom/internal/logger"
)

// acquireFromMMap enough executable space from holder
func acquireFromMMap(len int) (uintptr, *[]byte, error) {
	executableSpace, err := syscall.Mmap(
		-1,
		0,
		len,
		// W^X note (darwin/arm64):
		// RWX mappings can fail under macOS hardened runtime / JIT restrictions.
		// Allocate RW here, then switch to RX after writing the final code bytes.
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_ANON)
	if err != nil {
		logger.Debugf("acquireFromMMap fail: %v\n", err)
		return 0, nil, err
	}
	addr := (uintptr)(unsafe.Pointer(&executableSpace[0]))
	return addr, &executableSpace, nil
}
