//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump_test

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestUnitMakeDumpFunc_PreGo117_PrintsArgsAndReturnsZero(t *testing.T) {
	type Fn func(a int, b *int, c string) (int, *int)

	typ := reflect.TypeOf((Fn)(nil))
	fAny := argdump.MakeDumpFunc(typ)
	f, ok := fAny.(Fn)
	if !ok {
		t.Fatalf("type assertion failed: got %T", fAny)
	}

	x := 7
	out := captureStdout(t, func() {
		gotI, gotP := f(123, &x, "hi")
		if gotI != 0 {
			t.Fatalf("want int ret 0, got %d", gotI)
		}
		if gotP != nil {
			t.Fatalf("want ptr ret nil, got %v", gotP)
		}
	})

	if !strings.Contains(out, "arg0=") || !strings.Contains(out, "arg1=") || !strings.Contains(out, "arg2=") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}
