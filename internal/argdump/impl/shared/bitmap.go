//go:build go1.17 && (amd64 || arm64)
// +build go1.17
// +build amd64 arm64

package shared

import "github.com/tencent/goom/internal/argdump/abi"

// IntArgRegBitmap matches internal/abi.IntArgRegBitmap for tracking pointer registers.
type IntArgRegBitmap [(abi.IntArgRegs + 7) / 8]uint8

// Set marks the i-th register as containing a pointer.
func (b *IntArgRegBitmap) Set(i int) { b[i/8] |= uint8(1) << (i % 8) }

// Get returns whether the i-th register contains a pointer.
func (b *IntArgRegBitmap) Get(i int) bool {
	return b[i/8]&(uint8(1)<<(i%8)) != 0
}
