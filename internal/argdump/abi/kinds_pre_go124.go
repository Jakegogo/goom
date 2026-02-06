//go:build go1.17 && !go1.24
// +build go1.17,!go1.24

package abi

import "reflect"

// KindPointer is the reflect.Kind value for pointer types.
// In Go 1.17-1.23, this is reflect.Ptr.
const KindPointer = reflect.Ptr
