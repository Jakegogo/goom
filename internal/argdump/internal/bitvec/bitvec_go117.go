//go:build go1.17
// +build go1.17

package bitvec

import "github.com/tencent/goom/internal/argdump/abi"

// BitVector must agree with reflect.bitVector for go1.17+.
// Runtime reads {n uint32; bytedata *uint8} where bytedata points at the slice data.
type BitVector struct {
	N    uint32
	Data []byte
}

// Append adds a bit to the bitvector.
func (bv *BitVector) Append(bit uint8) {
	// Keep pointer masks aligned to uintptr boundaries (safe across versions).
	if bv.N%(8*uint32(abi.PtrSize)) == 0 {
		for i := uintptr(0); i < abi.PtrSize; i++ {
			bv.Data = append(bv.Data, 0)
		}
	}
	bv.Data[bv.N/8] |= bit << (bv.N % 8)
	bv.N++
}
