//go:build go1.24
// +build go1.24

package abi

import "reflect"

// KindPointer is the reflect.Kind value for pointer types.
// In Go 1.24+, this is reflect.Pointer (reflect.Ptr is deprecated).
const KindPointer = reflect.Pointer
