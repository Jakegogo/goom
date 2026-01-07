//go:build go1.24 && arm64

package abijson

import "unsafe"

// Minimal copies of Go 1.24 internal/abi structs/constants.
// These are copied to avoid importing stdlib internal packages and to avoid //go:linkname.

type abiKind uint8

const (
	abiKindInvalid abiKind = iota
	abiKindBool
	abiKindInt
	abiKindInt8
	abiKindInt16
	abiKindInt32
	abiKindInt64
	abiKindUint
	abiKindUint8
	abiKindUint16
	abiKindUint32
	abiKindUint64
	abiKindUintptr
	abiKindFloat32
	abiKindFloat64
	abiKindComplex64
	abiKindComplex128
	abiKindArray
	abiKindChan
	abiKindFunc
	abiKindInterface
	abiKindMap
	abiKindPointer
	abiKindSlice
	abiKindString
	abiKindStruct
	abiKindUnsafePointer
)

const (
	abiKindDirectIface abiKind = 1 << 5
	abiKindMask        abiKind = (1 << 5) - 1
)

type abiTFlag uint8

const (
	abiTFlagUncommon       abiTFlag = 1 << 0
	abiTFlagExtraStar      abiTFlag = 1 << 1
	abiTFlagNamed          abiTFlag = 1 << 2
	abiTFlagRegularMemory  abiTFlag = 1 << 3
	abiTFlagGCMaskOnDemand abiTFlag = 1 << 4
)

type abiNameOff int32
type abiTypeOff int32

// abiType matches go1.24 internal/abi.Type (prefix).
type abiType struct {
	Size_       uintptr
	PtrBytes    uintptr
	Hash        uint32
	TFlag       abiTFlag
	Align_      uint8
	FieldAlign_ uint8
	Kind_       abiKind
	Equal       func(unsafe.Pointer, unsafe.Pointer) bool
	GCData      *byte
	Str         abiNameOff
	PtrToThis   abiTypeOff
}

func (t *abiType) kind() abiKind    { return t.Kind_ & abiKindMask }
func (t *abiType) pointers() bool   { return t.PtrBytes != 0 }
func (t *abiType) ifaceIndir() bool { return t.Kind_&abiKindDirectIface == 0 }

// oldMapType matches go1.24 internal/abi.OldMapType.
type oldMapType struct {
	abiType
	Key        *abiType
	Elem       *abiType
	Bucket     *abiType
	Hasher     func(unsafe.Pointer, uintptr) uintptr
	KeySize    uint8
	ValueSize  uint8
	BucketSize uint16
	Flags      uint32
}

func (mt *oldMapType) indirectKey() bool  { return mt.Flags&1 != 0 }
func (mt *oldMapType) indirectElem() bool { return mt.Flags&2 != 0 }

const (
	oldMapBucketCountBits = 3
	oldMapBucketCount     = 1 << oldMapBucketCountBits
)

// swissMapType matches go1.24 internal/abi.SwissMapType.
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
