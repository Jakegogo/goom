//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

// NOTE: must agree with runtime.bitvector expectations for go1.13-go1.16.
// (Matches go1.16 reflect.bitVector.)
type bitVector struct {
	n    uint32
	data []byte
}

func (bv *bitVector) append(bit uint8) {
	if bv.n%8 == 0 {
		bv.data = append(bv.data, 0)
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}
