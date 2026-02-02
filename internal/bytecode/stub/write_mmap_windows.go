//go:build windows
// +build windows

package stub

// On Windows, the mmap path is expected to produce an executable mapping.
// Keep the write path as a plain copy.
func writeToMMap(_ uintptr, space *[]byte, data []byte) error {
	copy(*space, data)
	return nil
}
