//go:build go1.24 && arm64

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

func TestMakeDumpFunc_PrintsArgsAndReturnsZero(t *testing.T) {
	type Fn func(a int, b *int, c string, d float32) (int, *int)

	typ := reflect.TypeOf((Fn)(nil))
	fAny := argdump.MakeDumpFunc(typ)
	f, ok := fAny.(Fn)
	if !ok {
		t.Fatalf("type assertion failed: got %T", fAny)
	}

	// Capture stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	x := 7
	gotI, gotP := f(123, &x, "hi", 3.5)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if gotI != 0 {
		t.Fatalf("want int ret 0, got %d", gotI)
	}
	if gotP != nil {
		t.Fatalf("want ptr ret nil, got %v", gotP)
	}

	out := buf.String()
	if os.Getenv("ARGDUMP_DEBUG") == "1" {
		_, _ = os.Stderr.WriteString("dump:\n" + out + "\n")
	}
	if !strings.Contains(out, "arg0=") || !strings.Contains(out, "arg1=") || !strings.Contains(out, "arg2=") || !strings.Contains(out, "arg3=") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}
