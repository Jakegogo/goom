package util

import (
	"reflect"
	"unsafe"
)

// Eface matches the runtime internal empty interface layout.
type Eface struct {
	Typ  unsafe.Pointer
	Data unsafe.Pointer
}

// PackEface packs a type pointer and data pointer into an interface{}.
func PackEface(typ, data unsafe.Pointer) interface{} {
	e := Eface{Typ: typ, Data: data}
	return *(*interface{})(unsafe.Pointer(&e))
}

// RtypePtr extracts the *rtype pointer from a reflect.Type interface.
func RtypePtr(t reflect.Type) unsafe.Pointer {
	// reflect.Type is an interface. Its data word is *rtype.
	type iface struct {
		tab  unsafe.Pointer
		data unsafe.Pointer
	}
	return (*iface)(unsafe.Pointer(&t)).data
}

// Align rounds x up to a multiple of a.
func Align(x, a uintptr) uintptr { return (x + a - 1) &^ (a - 1) }
