//go:build go1.13 && !goexperiment.swissmap
// +build go1.13,!goexperiment.swissmap

package abijson

import (
	"reflect"
	"strconv"
	"unsafe"
)

func (e *encoder) encodeMap(t reflect.Type, addr unsafe.Pointer, depth int) error {
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
