//go:build go1.24 && arm64 && goexperiment.swissmap

package abijson

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestEncodeJSONFromAddr_SwissMap(t *testing.T) {
	i := 123
	v := S{
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
	t.Logf("json: %s", string(b))
}

// Keep the same name as the noswiss test so IDE "Run Test" commands work
// regardless of swissmap experiment build tags.
func TestEncodeJSONFromAddr_Basic(t *testing.T) {
	TestEncodeJSONFromAddr_SwissMap(t)
}
