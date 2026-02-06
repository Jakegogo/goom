//go:build go1.24 && goexperiment.swissmap && (amd64 || arm64 || 386)
// +build go1.24
// +build goexperiment.swissmap
// +build amd64 arm64 386

package abijson

import "unsafe"

// swissMapType matches the runtime swiss map type shape used by the iterator.
// This is intentionally minimal and best-effort.
type swissMapType struct {
	abiType
	Key       *abiType
	Elem      *abiType
	Group     *abiType
	Hasher    func(unsafe.Pointer, uintptr) uintptr
	GroupSize uintptr
	SlotSize  uintptr
	ElemOff   uintptr
	Flags     uint32
}

const (
	swissMapNeedKeyUpdate uint32 = 1 << iota
	swissMapHashMightPanic
	swissMapIndirectKey
	swissMapIndirectElem
)

func (mt *swissMapType) indirectKey() bool  { return mt.Flags&swissMapIndirectKey != 0 }
func (mt *swissMapType) indirectElem() bool { return mt.Flags&swissMapIndirectElem != 0 }

const (
	swissMapGroupSlotsBits = 3
	swissMapGroupSlots     = 1 << swissMapGroupSlotsBits // 8
)
