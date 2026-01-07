//go:build go1.24 && arm64 && goexperiment.swissmap

package abijson

import (
	"errors"
	"reflect"
	"strconv"
	"unsafe"
)

// Minimal swissmap iterator forked from go1.24 internal/runtime/maps layouts.
// This is a POC: it assumes no concurrent map writes and does not attempt to
// preserve spec iteration semantics under mutation/grow.

const (
	swissCtrlEmpty   uint8 = 0x80
	swissCtrlDeleted uint8 = 0xFE
)

type swissMap struct {
	used        uint64
	seed        uintptr
	dirPtr      unsafe.Pointer
	dirLen      int
	globalDepth uint8
	globalShift uint8
	writing     uint8
	_           uint8 // padding
	clearSeq    uint64
}

type swissTable struct {
	used       uint16
	capacity   uint16
	growthLeft uint16
	localDepth uint8
	_          uint8 // padding so index is 8-aligned on arm64
	index      int
	groups     swissGroups
}

type swissGroups struct {
	data       unsafe.Pointer // *[length]group
	lengthMask uint64
}

type swissGroupRef struct {
	data unsafe.Pointer // *group
}

type swissCtrlGroup uint64

func (g *swissGroupRef) ctrls() *swissCtrlGroup { return (*swissCtrlGroup)(g.data) }

func (g *swissGroupRef) key(mt *swissMapType, i uintptr) unsafe.Pointer {
	// group layout: ctrls uint64; slots[8]slot
	return unsafe.Pointer(uintptr(g.data) + unsafe.Sizeof(swissCtrlGroup(0)) + i*mt.SlotSize)
}

func (g *swissGroupRef) elem(mt *swissMapType, i uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(g.data) + unsafe.Sizeof(swissCtrlGroup(0)) + i*mt.SlotSize + mt.ElemOff)
}

func (gs *swissGroups) group(mt *swissMapType, i uint64) swissGroupRef {
	return swissGroupRef{data: unsafe.Pointer(uintptr(gs.data) + uintptr(i)*mt.GroupSize)}
}

func (cg *swissCtrlGroup) get(i uintptr) uint8 {
	// arm64 darwin is little-endian
	return *(*uint8)(unsafe.Add(unsafe.Pointer(cg), i))
}

type swissMapIter struct {
	mt *swissMapType
	m  *swissMap

	// current mode
	small      bool
	smallGroup swissGroupRef
	smallSlot  uintptr

	// directory iteration
	dirIdx   int
	curTable *swissTable
	groupIdx uint64
	slotIdx  uintptr

	visited map[uintptr]struct{}
}

func mapHeaderSwiss(mapAddr unsafe.Pointer) *swissMap {
	p := *(*unsafe.Pointer)(mapAddr)
	if p == nil {
		return nil
	}
	return (*swissMap)(p)
}

func newSwissIter(mt *swissMapType, m *swissMap) (*swissMapIter, error) {
	if m == nil || m.used == 0 {
		return &swissMapIter{mt: mt, m: m}, nil
	}
	if m.writing != 0 {
		return nil, errors.New("concurrent map iteration and map write")
	}
	it := &swissMapIter{mt: mt, m: m}
	if m.dirLen <= 0 {
		it.small = true
		it.smallGroup = swissGroupRef{data: m.dirPtr}
		return it, nil
	}
	it.visited = make(map[uintptr]struct{}, 8)
	return it, nil
}

// next returns (keyAddr, elemAddr, ok).
func (it *swissMapIter) next() (unsafe.Pointer, unsafe.Pointer, bool) {
	if it.m == nil || it.m.used == 0 {
		return nil, nil, false
	}
	if it.small {
		for it.smallSlot < swissMapGroupSlots {
			i := it.smallSlot
			it.smallSlot++
			c := it.smallGroup.ctrls().get(i)
			if c&swissCtrlEmpty == swissCtrlEmpty || c == swissCtrlDeleted {
				continue
			}
			k := it.smallGroup.key(it.mt, i)
			if it.mt.indirectKey() {
				k = *(*unsafe.Pointer)(k)
			}
			e := it.smallGroup.elem(it.mt, i)
			if it.mt.indirectElem() {
				e = *(*unsafe.Pointer)(e)
			}
			return k, e, true
		}
		return nil, nil, false
	}

	for {
		if it.curTable == nil {
			// advance to next unique table
			for it.dirIdx < it.m.dirLen {
				tp := *(*unsafe.Pointer)(unsafe.Pointer(uintptr(it.m.dirPtr) + uintptr(it.dirIdx)*unsafe.Sizeof(uintptr(0))))
				it.dirIdx++
				if tp == nil {
					continue
				}
				if _, ok := it.visited[uintptr(tp)]; ok {
					continue
				}
				it.visited[uintptr(tp)] = struct{}{}
				it.curTable = (*swissTable)(tp)
				it.groupIdx = 0
				it.slotIdx = 0
				break
			}
			if it.curTable == nil {
				return nil, nil, false
			}
		}

		gs := it.curTable.groups
		for it.groupIdx <= gs.lengthMask {
			g := gs.group(it.mt, it.groupIdx)
			// iterate slots
			for it.slotIdx < swissMapGroupSlots {
				si := it.slotIdx
				it.slotIdx++
				c := g.ctrls().get(si)
				if c&swissCtrlEmpty == swissCtrlEmpty || c == swissCtrlDeleted {
					continue
				}
				k := g.key(it.mt, si)
				if it.mt.indirectKey() {
					k = *(*unsafe.Pointer)(k)
				}
				e := g.elem(it.mt, si)
				if it.mt.indirectElem() {
					e = *(*unsafe.Pointer)(e)
				}
				return k, e, true
			}
			it.groupIdx++
			it.slotIdx = 0
		}
		// done with this table
		it.curTable = nil
	}
}

// encodeMap implementation for swissmap toolchains.
func (e *encoder) encodeMap(t reflect.Type, addr unsafe.Pointer, depth int) error {
	mt := (*swissMapType)(unsafe.Pointer(rtypePtr(t)))
	m := mapHeaderSwiss(addr)
	if m == nil || m.used == 0 {
		e.buf.WriteString("{}")
		return nil
	}

	it, err := newSwissIter(mt, m)
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
	if int(m.used) > wrote {
		e.buf.WriteByte(',')
		e.writeString("<truncated>")
		e.buf.WriteByte(':')
		e.buf.WriteString(strconv.FormatInt(int64(int(m.used)-wrote), 10))
	}
	e.buf.WriteByte('}')
	return nil
}

func (e *encoder) encodeMapKeyToString(t reflect.Type, addr unsafe.Pointer, depth int) (string, error) {
	// reuse the same helper as noswiss by delegating through JSON encoding for uncommon keys.
	switch t.Kind() {
	case reflect.String:
		return *(*string)(addr), nil
	case reflect.Int:
		return strconvI64(int64(*(*int)(addr))), nil
	case reflect.Int8:
		return strconvI64(int64(*(*int8)(addr))), nil
	case reflect.Int16:
		return strconvI64(int64(*(*int16)(addr))), nil
	case reflect.Int32:
		return strconvI64(int64(*(*int32)(addr))), nil
	case reflect.Int64:
		return strconvI64(*(*int64)(addr)), nil
	case reflect.Uint:
		return strconvU64(uint64(*(*uint)(addr))), nil
	case reflect.Uint8:
		return strconvU64(uint64(*(*uint8)(addr))), nil
	case reflect.Uint16:
		return strconvU64(uint64(*(*uint16)(addr))), nil
	case reflect.Uint32:
		return strconvU64(uint64(*(*uint32)(addr))), nil
	case reflect.Uint64:
		return strconvU64(*(*uint64)(addr)), nil
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

func strconvI64(v int64) string { return strconv.FormatInt(v, 10) }

func strconvU64(v uint64) string { return strconv.FormatUint(v, 10) }
