//go:build go1.24 && arm64 && !goexperiment.swissmap

package abijson

import (
	"reflect"
	"strconv"
	"unsafe"
)

func (e *encoder) encodeMap(t reflect.Type, addr unsafe.Pointer, depth int) error {
	// Go1.24 noswiss map layout iterator.
	mt := (*oldMapType)(unsafe.Pointer(rtypePtr(t)))
	h := mapHeaderPtr(addr)
	if h == nil || h.count == 0 {
		e.buf.WriteString("{}")
		return nil
	}
	it, err := newMapIter(mt, h)
	if err != nil {
		e.writeString("<map_unavailable>")
		return nil
	}

	keyT := t.Key()
	valT := t.Elem()

	e.buf.WriteByte('{')
	wrote := 0
	for {
		kp, vp, ok := it.next()
		if !ok {
			break
		}
		if wrote >= e.opt.MaxMapItems {
			break
		}
		if wrote > 0 {
			e.buf.WriteByte(',')
		}
		keyStr, kerr := e.encodeMapKeyToString(keyT, kp, depth+1)
		if kerr != nil {
			return kerr
		}
		e.writeString(keyStr)
		e.buf.WriteByte(':')
		if err := e.encode(valT, vp, depth+1); err != nil {
			return err
		}
		wrote++
	}
	if h.count > wrote {
		e.buf.WriteByte(',')
		e.writeString("<truncated>")
		e.buf.WriteByte(':')
		e.buf.WriteString(strconv.FormatInt(int64(h.count-wrote), 10))
	}
	e.buf.WriteByte('}')
	return nil
}

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
		// Encode the key as JSON then use its JSON as a string key.
		b, err := EncodeJSONFromAddrWithOptions(t, addr, e.opt)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
