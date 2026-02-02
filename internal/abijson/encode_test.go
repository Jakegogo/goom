//go:build go1.13
// +build go1.13

package abijson

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

type testS struct {
	A int            `json:"a"`
	B string         `json:"b"`
	M map[string]int `json:"m"`
	X interface{}    `json:"x"`
	P *int           `json:"p"`
}

func TestEncodeJSONFromAddr_Basic(t *testing.T) {
	i := 123
	v := testS{
		A: 7,
		B: "ok",
		M: map[string]int{"k": 1, "z": 2},
		X: map[string]int{"a": 9},
		P: &i,
	}

	b, err := EncodeJSONFromAddr(reflect.TypeOf(v), unsafe.Pointer(&v))
	if err != nil {
		t.Fatalf("EncodeJSONFromAddr error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("empty json")
	}
	fmt.Println(string(b))
}
