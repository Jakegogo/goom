package argdump

import (
	"reflect"
	"unsafe"
)

// eface matches the runtime internal empty interface layout.
type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

// packEface packs a type pointer and data pointer into an interface{}.
func packEface(typ, data unsafe.Pointer) interface{} {
	e := eface{typ: typ, data: data}
	return *(*interface{})(unsafe.Pointer(&e))
}

// rtypePtr extracts the *rtype pointer from a reflect.Type interface.
func rtypePtr(t reflect.Type) unsafe.Pointer {
	// reflect.Type is an interface. Its data word is *rtype.
	type iface struct {
		tab  unsafe.Pointer
		data unsafe.Pointer
	}
	return (*iface)(unsafe.Pointer(&t)).data
}

// align rounds x up to a multiple of a.
func align(x, a uintptr) uintptr { return (x + a - 1) &^ (a - 1) }
