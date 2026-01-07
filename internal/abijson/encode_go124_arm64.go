//go:build go1.24 && arm64

package abijson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"unicode/utf8"
	"unsafe"
)

var ErrUnsupported = errors.New("abijson: unsupported type")

type Options struct {
	MaxDepth     int
	MaxSliceLen  int
	MaxMapItems  int
	InvalidPtrAs string // marker string for invalid pointers
}

func DefaultOptions() Options {
	return Options{
		MaxDepth:     30,
		MaxSliceLen:  10_000,
		MaxMapItems:  100_000,
		InvalidPtrAs: "<invalid_ptr>",
	}
}

// EncodeJSONFromAddr encodes a value of type t located at addr into JSON.
// It does not use reflect.Value. It reads memory directly via unsafe.
func EncodeJSONFromAddr(t reflect.Type, addr unsafe.Pointer) ([]byte, error) {
	return EncodeJSONFromAddrWithOptions(t, addr, DefaultOptions())
}

func EncodeJSONFromAddrWithOptions(t reflect.Type, addr unsafe.Pointer, opt Options) ([]byte, error) {
	var b bytes.Buffer
	enc := encoder{opt: opt, buf: &b}
	if err := enc.encode(t, addr, 0); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

type encoder struct {
	opt Options
	buf *bytes.Buffer
}

func (e *encoder) encode(t reflect.Type, addr unsafe.Pointer, depth int) error {
	if depth > e.opt.MaxDepth {
		e.writeString("<max_depth>")
		return nil
	}
	switch t.Kind() {
	case reflect.Bool:
		if *(*bool)(addr) {
			e.buf.WriteString("true")
		} else {
			e.buf.WriteString("false")
		}
		return nil
	case reflect.Int:
		e.buf.WriteString(strconv.FormatInt(int64(*(*int)(addr)), 10))
		return nil
	case reflect.Int8:
		e.buf.WriteString(strconv.FormatInt(int64(*(*int8)(addr)), 10))
		return nil
	case reflect.Int16:
		e.buf.WriteString(strconv.FormatInt(int64(*(*int16)(addr)), 10))
		return nil
	case reflect.Int32:
		e.buf.WriteString(strconv.FormatInt(int64(*(*int32)(addr)), 10))
		return nil
	case reflect.Int64:
		e.buf.WriteString(strconv.FormatInt(*(*int64)(addr), 10))
		return nil
	case reflect.Uint:
		e.buf.WriteString(strconv.FormatUint(uint64(*(*uint)(addr)), 10))
		return nil
	case reflect.Uint8:
		e.buf.WriteString(strconv.FormatUint(uint64(*(*uint8)(addr)), 10))
		return nil
	case reflect.Uint16:
		e.buf.WriteString(strconv.FormatUint(uint64(*(*uint16)(addr)), 10))
		return nil
	case reflect.Uint32:
		e.buf.WriteString(strconv.FormatUint(uint64(*(*uint32)(addr)), 10))
		return nil
	case reflect.Uint64:
		e.buf.WriteString(strconv.FormatUint(*(*uint64)(addr), 10))
		return nil
	case reflect.Uintptr:
		e.buf.WriteString(strconv.FormatUint(uint64(*(*uintptr)(addr)), 10))
		return nil
	case reflect.Float32:
		e.buf.WriteString(strconv.FormatFloat(float64(*(*float32)(addr)), 'g', -1, 32))
		return nil
	case reflect.Float64:
		e.buf.WriteString(strconv.FormatFloat(*(*float64)(addr), 'g', -1, 64))
		return nil
	case reflect.Complex64:
		c := *(*complex64)(addr)
		e.buf.WriteByte('"')
		e.buf.WriteString(strconv.FormatFloat(float64(real(c)), 'g', -1, 32))
		e.buf.WriteByte('+')
		e.buf.WriteString(strconv.FormatFloat(float64(imag(c)), 'g', -1, 32))
		e.buf.WriteByte('i')
		e.buf.WriteByte('"')
		return nil
	case reflect.Complex128:
		c := *(*complex128)(addr)
		e.buf.WriteByte('"')
		e.buf.WriteString(strconv.FormatFloat(real(c), 'g', -1, 64))
		e.buf.WriteByte('+')
		e.buf.WriteString(strconv.FormatFloat(imag(c), 'g', -1, 64))
		e.buf.WriteByte('i')
		e.buf.WriteByte('"')
		return nil
	case reflect.String:
		s := *(*string)(addr)
		e.writeString(s)
		return nil
	case reflect.Slice:
		return e.encodeSlice(t, addr, depth)
	case reflect.Array:
		return e.encodeArray(t, addr, depth)
	case reflect.Struct:
		return e.encodeStruct(t, addr, depth)
	case reflect.Pointer:
		return e.encodePointer(t, addr, depth)
	case reflect.Interface:
		return e.encodeInterface(t, addr, depth)
	case reflect.Map:
		return e.encodeMap(t, addr, depth)
	default:
		return fmt.Errorf("%w: kind=%s type=%s", ErrUnsupported, t.Kind(), t.String())
	}
}

func (e *encoder) encodePointer(t reflect.Type, addr unsafe.Pointer, depth int) error {
	p := *(*unsafe.Pointer)(addr)
	if p == nil {
		e.buf.WriteString("null")
		return nil
	}
	// Best-effort pointer sanity check to avoid obvious segfaults.
	up := uintptr(p)
	if up < 0x10000 || up%uintptr(t.Elem().Align()) != 0 {
		e.writeString(e.opt.InvalidPtrAs)
		return nil
	}
	return e.encode(t.Elem(), p, depth+1)
}

func (e *encoder) encodeInterface(t reflect.Type, addr unsafe.Pointer, depth int) error {
	dt, data, ok := readInterface(t, addr)
	if !ok {
		e.buf.WriteString("null")
		return nil
	}
	// If dynamic type is direct-in-iface and data points to a temporary on stack,
	// encode synchronously and do not store it.
	return e.encode(dt, data, depth+1)
}

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func (e *encoder) encodeSlice(t reflect.Type, addr unsafe.Pointer, depth int) error {
	h := *(*sliceHeader)(addr)
	if h.Data == nil {
		e.buf.WriteString("null")
		return nil
	}
	n := h.Len
	if n > e.opt.MaxSliceLen {
		n = e.opt.MaxSliceLen
	}
	elem := t.Elem()
	elemSize := elem.Size()

	e.buf.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			e.buf.WriteByte(',')
		}
		ep := unsafe.Pointer(uintptr(h.Data) + uintptr(i)*elemSize)
		if err := e.encode(elem, ep, depth+1); err != nil {
			return err
		}
	}
	if h.Len > n {
		e.buf.WriteString(",")
		e.writeString("<truncated>")
	}
	e.buf.WriteByte(']')
	return nil
}

func (e *encoder) encodeArray(t reflect.Type, addr unsafe.Pointer, depth int) error {
	n := t.Len()
	if n > e.opt.MaxSliceLen {
		n = e.opt.MaxSliceLen
	}
	elem := t.Elem()
	elemSize := elem.Size()
	e.buf.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			e.buf.WriteByte(',')
		}
		ep := unsafe.Pointer(uintptr(addr) + uintptr(i)*elemSize)
		if err := e.encode(elem, ep, depth+1); err != nil {
			return err
		}
	}
	if t.Len() > n {
		e.buf.WriteString(",")
		e.writeString("<truncated>")
	}
	e.buf.WriteByte(']')
	return nil
}

func (e *encoder) encodeStruct(t reflect.Type, addr unsafe.Pointer, depth int) error {
	e.buf.WriteByte('{')
	n := t.NumField()
	first := true
	for i := 0; i < n; i++ {
		f := t.Field(i)
		if tag := f.Tag.Get("json"); tag == "-" {
			continue
		}
		name := f.Name
		if tag := f.Tag.Get("json"); tag != "" {
			if comma := indexComma(tag); comma >= 0 {
				if comma > 0 {
					name = tag[:comma]
				}
			} else {
				name = tag
			}
			if name == "" {
				name = f.Name
			}
		}
		if !first {
			e.buf.WriteByte(',')
		}
		first = false
		e.writeString(name)
		e.buf.WriteByte(':')
		fp := unsafe.Pointer(uintptr(addr) + f.Offset)
		if err := e.encode(f.Type, fp, depth+1); err != nil {
			return err
		}
	}
	e.buf.WriteByte('}')
	return nil
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (e *encoder) writeString(s string) {
	// Avoid reflect.Value; use json escaping logic via json.Marshal on string.
	// This allocates but only for bytes, not pointer graphs.
	b, _ := json.Marshal(s)
	e.buf.Write(b)
}

// utf8IsValid reports whether s is valid UTF-8.
func utf8IsValid(s string) bool { return utf8.ValidString(s) }
