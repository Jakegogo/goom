package util

import (
	"reflect"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/internal/bitvec"
)

// TypeHasPointers returns true if the type contains pointer fields.
func TypeHasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, abi.KindPointer, reflect.Slice, reflect.String,
		reflect.Interface, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return TypeHasPointers(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if TypeHasPointers(t.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// AddTypeBits adds pointer bits to the bitvector for the given type at the given offset.
func AddTypeBits(bv *bitvec.BitVector, offset uintptr, t reflect.Type) {
	if !TypeHasPointers(t) {
		return
	}
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, abi.KindPointer, reflect.Slice, reflect.String, reflect.UnsafePointer:
		for bv.N < uint32(offset/abi.PtrSize) {
			bv.Append(0)
		}
		bv.Append(1)
	case reflect.Interface:
		for bv.N < uint32(offset/abi.PtrSize) {
			bv.Append(0)
		}
		bv.Append(1)
		bv.Append(1)
	case reflect.Array:
		for i := 0; i < t.Len(); i++ {
			AddTypeBits(bv, offset+uintptr(i)*t.Elem().Size(), t.Elem())
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			AddTypeBits(bv, offset+f.Offset, f.Type)
		}
	}
}
