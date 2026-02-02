//go:build go1.13
// +build go1.13

package abijson

import (
	"reflect"
	"strconv"
	"unsafe"
)

// encodeMapKeyToString renders a map key as a JSON object key string.
// For "simple" scalar keys we avoid allocations. For others, we encode the key
// value to JSON and use that JSON as the key string.
func (e *encoder) encodeMapKeyToString(t reflect.Type, addr unsafe.Pointer, depth int) (string, error) {
	switch t.Kind() {
	case reflect.String:
		return *(*string)(addr), nil
	case reflect.Int:
		return strconv.FormatInt(int64(*(*int)(addr)), 10), nil
	case reflect.Int8:
		return strconv.FormatInt(int64(*(*int8)(addr)), 10), nil
	case reflect.Int16:
		return strconv.FormatInt(int64(*(*int16)(addr)), 10), nil
	case reflect.Int32:
		return strconv.FormatInt(int64(*(*int32)(addr)), 10), nil
	case reflect.Int64:
		return strconv.FormatInt(*(*int64)(addr), 10), nil
	case reflect.Uint:
		return strconv.FormatUint(uint64(*(*uint)(addr)), 10), nil
	case reflect.Uint8:
		return strconv.FormatUint(uint64(*(*uint8)(addr)), 10), nil
	case reflect.Uint16:
		return strconv.FormatUint(uint64(*(*uint16)(addr)), 10), nil
	case reflect.Uint32:
		return strconv.FormatUint(uint64(*(*uint32)(addr)), 10), nil
	case reflect.Uint64:
		return strconv.FormatUint(*(*uint64)(addr), 10), nil
	case reflect.Bool:
		if *(*bool)(addr) {
			return "true", nil
		}
		return "false", nil
	default:
		b, err := EncodeJSONFromAddrWithOptions(t, addr, e.opt)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
