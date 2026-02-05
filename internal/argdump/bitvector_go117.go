//go:build go1.17
// +build go1.17

package argdump

// bitVector must agree with reflect.bitVector for go1.17+.
// Runtime reads {n uint32; bytedata *uint8} where bytedata points at the slice data.
type bitVector struct {
	n    uint32
	data []byte
}

func (bv *bitVector) append(bit uint8) {
	// Keep pointer masks aligned to uintptr boundaries (safe across versions).
	if bv.n%(8*uint32(ptrSize)) == 0 {
		for i := uintptr(0); i < ptrSize; i++ {
			bv.data = append(bv.data, 0)
		}
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}
