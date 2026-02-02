//go:build go1.13 && !go1.17 && amd64
// +build go1.13,!go1.17,amd64

package argdump_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
)

func TestUnitMakeDumpFunc_PreGo117_AMD64_FloatsAndMultiReturn(t *testing.T) {
	type Fn func(a int32, b float64, c float32, d string) (int64, float32, *int)

	typ := reflect.TypeOf((Fn)(nil))
	fAny := argdump.MakeDumpFunc(typ)
	f, ok := fAny.(Fn)
	if !ok {
		t.Fatalf("type assertion failed: got %T", fAny)
	}

	out := captureStdout(t, func() {
		gotI64, gotF32, gotP := f(7, 2.5, 3.75, "ok")
		if gotI64 != 0 {
			t.Fatalf("want int64 ret 0, got %d", gotI64)
		}
		if gotF32 != 0 {
			t.Fatalf("want float32 ret 0, got %v", gotF32)
		}
		if gotP != nil {
			t.Fatalf("want ptr ret nil, got %v", gotP)
		}
	})

	// Ensure all args were dumped.
	for _, k := range []string{"arg0=", "arg1=", "arg2=", "arg3="} {
		if !strings.Contains(out, k) {
			t.Fatalf("missing %q in stdout:\n%s", k, out)
		}
	}
}
