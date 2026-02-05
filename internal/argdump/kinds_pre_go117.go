//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

import "reflect"

const kindPointer = reflect.Ptr
