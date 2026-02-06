//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package bitvec

// NOTE: must agree with runtime.bitvector expectations for go1.13-go1.16.
// (Matches go1.16 reflect.bitVector.)

// BitVector tracks pointer locations in a memory region.
type BitVector struct {
	N    uint32
	Data []byte
}

// Append adds a bit to the bitvector.
func (bv *BitVector) Append(bit uint8) {
	if bv.N%8 == 0 {
		bv.Data = append(bv.Data, 0)
	}
	bv.Data[bv.N/8] |= bit << (bv.N % 8)
	bv.N++
}
