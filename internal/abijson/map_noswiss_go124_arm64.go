//go:build go1.24 && arm64 && !goexperiment.swissmap

package abijson

import (
	"errors"
	"unsafe"
)

// This file is a minimal, best-effort map iterator for Go 1.24 noswiss map layout.
// It is adapted from Go 1.24 runtime/map_noswiss.go, but trimmed down and without //go:linkname.

const (
	emptyRest  = 0
	emptyOne   = 1
	minTopHash = 5
)

func isEmptyTopHash(x uint8) bool { return x <= emptyOne }

// hmap matches Go 1.24 runtime.hmap layout for !swissmap.
type hmap struct {
	count     int
	flags     uint8
	B         uint8
	noverflow uint16
	hash0     uint32

	buckets    unsafe.Pointer
	oldbuckets unsafe.Pointer
	nevacuate  uintptr
	clearSeq   uint64

	extra unsafe.Pointer // *mapextra (not used in this POC)
}

// bmap is bucket header (tophash only; data follows).
type bmap struct {
	tophash [oldMapBucketCount]uint8
}

// dataOffset matches runtime's computed dataOffset (aligned).
var dataOffset = unsafe.Offsetof(struct {
	b bmap
	v int64
}{}.v)

func add(p unsafe.Pointer, x uintptr) unsafe.Pointer { return unsafe.Add(p, x) }

func bucketShift(b uint8) uintptr { return uintptr(1) << (b & 63) }

func (b *bmap) overflow(t *oldMapType) *bmap {
	// overflow pointer is at end of bucket.
	return *(**bmap)(add(unsafe.Pointer(b), uintptr(t.BucketSize)-unsafe.Sizeof(uintptr(0))))
}

// mapIter walks over all buckets and overflow buckets.
// This iterator assumes the map is not concurrently written and not in the middle of growth.
type mapIter struct {
	mt   *oldMapType
	h    *hmap
	nb   uintptr
	bi   uintptr
	b    *bmap
	off  int
	done bool
}

func newMapIter(mt *oldMapType, h *hmap) (*mapIter, error) {
	if h == nil || h.count == 0 {
		return &mapIter{done: true}, nil
	}
	if h.oldbuckets != nil {
		// Full correctness during growth requires the full runtime algorithm (mapaccessK etc.).
		// POC refuses to avoid duplicates/misses.
		return nil, errors.New("map is growing (oldbuckets != nil): not supported in POC")
	}
	nb := bucketShift(h.B)
	return &mapIter{mt: mt, h: h, nb: nb, bi: 0, b: nil, off: 0}, nil
}

// next returns (keyAddr, valAddr, ok).
func (it *mapIter) next() (unsafe.Pointer, unsafe.Pointer, bool) {
	if it.done {
		return nil, nil, false
	}
	for {
		if it.b == nil {
			if it.bi >= it.nb {
				it.done = true
				return nil, nil, false
			}
			it.b = (*bmap)(add(it.h.buckets, it.bi*uintptr(it.mt.BucketSize)))
			it.off = 0
			it.bi++
		}

		for it.off < oldMapBucketCount {
			i := it.off
			it.off++
			th := it.b.tophash[i]
			if isEmptyTopHash(th) {
				continue
			}

			k := add(unsafe.Pointer(it.b), dataOffset+uintptr(i)*uintptr(it.mt.KeySize))
			if it.mt.indirectKey() {
				k = *(*unsafe.Pointer)(k)
			}

			e := add(unsafe.Pointer(it.b),
				dataOffset+oldMapBucketCount*uintptr(it.mt.KeySize)+uintptr(i)*uintptr(it.mt.ValueSize))
			if it.mt.indirectElem() {
				e = *(*unsafe.Pointer)(e)
			}
			return k, e, true
		}

		// move to overflow
		it.b = it.b.overflow(it.mt)
		it.off = 0
	}
}

func mapHeaderPtr(mapAddr unsafe.Pointer) *hmap {
	// A map value is a pointer to hmap.
	p := *(*unsafe.Pointer)(mapAddr)
	if p == nil {
		return nil
	}
	return (*hmap)(p)
}
