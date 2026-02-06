//go:build go1.13
// +build go1.13

package abijson

import "unsafe"

// Minimal copies of runtime type structs/constants (stable enough for go1.13+).
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

type abiNameOff int32
type abiTypeOff int32

// abiType matches the runtime type prefix used by reflect.Type's concrete representation.
// (In newer toolchains this aligns with internal/abi.Type.)
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

// oldMapType matches the runtime noswiss map type used by the compiler/runtime.
// Layout is treated as a best-effort POC and may drift across toolchains.
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
