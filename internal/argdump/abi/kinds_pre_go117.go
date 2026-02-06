//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package abi

import "reflect"

// KindPointer is the reflect.Kind value for pointer types.
// In Go 1.13-1.16, this is reflect.Ptr.
const KindPointer = reflect.Ptr
