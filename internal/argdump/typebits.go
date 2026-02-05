package argdump

import "reflect"

func typeHasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, kindPointer, reflect.Slice, reflect.String,
		reflect.Interface, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return typeHasPointers(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if typeHasPointers(t.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func addTypeBits(bv *bitVector, offset uintptr, t reflect.Type) {
	if !typeHasPointers(t) {
		return
	}
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, kindPointer, reflect.Slice, reflect.String, reflect.UnsafePointer:
		for bv.n < uint32(offset/ptrSize) {
			bv.append(0)
		}
		bv.append(1)
	case reflect.Interface:
		for bv.n < uint32(offset/ptrSize) {
			bv.append(0)
		}
		bv.append(1)
		bv.append(1)
	case reflect.Array:
		for i := 0; i < t.Len(); i++ {
			addTypeBits(bv, offset+uintptr(i)*t.Elem().Size(), t.Elem())
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			addTypeBits(bv, offset+f.Offset, f.Type)
		}
	}
}
