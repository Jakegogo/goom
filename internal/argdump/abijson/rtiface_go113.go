//go:build go1.13
// +build go1.13

package abijson

import (
	"reflect"
	"unsafe"
)

// Low-level interface headers (Go ABI).
// eface: empty interface, iface: non-empty interface.
type eface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

type iface struct {
	tab  unsafe.Pointer
	data unsafe.Pointer
}

// rtypePtr returns the runtime type pointer (*abiType) backing a reflect.Type.
func rtypePtr(t reflect.Type) *abiType {
	return (*abiType)((*iface)(unsafe.Pointer(&t)).data)
}

// reflectTypeFromRType builds a reflect.Type from a runtime type pointer.
// This avoids reflect.Value and avoids any linkname.
func reflectTypeFromRType(rt *abiType) reflect.Type {
	// Any reflect.Type value gives us the correct itab for the concrete type (*rtype / *abi.Type).
	var sample reflect.Type = reflect.TypeOf(int(0))
	it := (*iface)(unsafe.Pointer(&sample)).tab
	out := iface{tab: it, data: unsafe.Pointer(rt)}
	return *(*reflect.Type)(unsafe.Pointer(&out))
}

// readInterface decodes an interface value located at addr with static type it (which must be Kind Interface).
// It returns (dynamicType, dataAddr, ok).
func readInterface(it reflect.Type, addr unsafe.Pointer) (reflect.Type, unsafe.Pointer, bool) {
	// Determine whether this interface is empty or non-empty from the static type.
	// - empty interface: eface{typ,data}
	// - non-empty interface: iface{itab,data}
	if it.NumMethod() == 0 {
		ev := *(*eface)(addr)
		if ev.typ == nil {
			return nil, nil, false
		}
		dt := (*abiType)(ev.typ)
		dynamic := reflectTypeFromRType(dt)

		// For direct iface values, ev.data is the data word, not a pointer to storage.
		// Encode synchronously via a temporary stack slot.
		if !dt.ifaceIndir() {
			var tmp uintptr = uintptr(ev.data)
			return dynamic, unsafe.Pointer(&tmp), true
		}
		return dynamic, ev.data, true
	}

	iv := *(*iface)(addr)
	if iv.tab == nil {
		return nil, nil, false
	}

	// itab layout is runtime-private; it has (inter, _type, hash, ...) so _type is at +ptrSize.
	typPtr := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(iv.tab) + unsafe.Sizeof(uintptr(0))))
	if typPtr == nil {
		return nil, nil, false
	}
	dt := (*abiType)(typPtr)
	dynamic := reflectTypeFromRType(dt)
	return dynamic, iv.data, true
}
