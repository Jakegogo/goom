//go:build go1.18 && !go1.24 && amd64
// +build go1.18,!go1.24,amd64

package argdump_test

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
	"github.com/tencent/goom/internal/bytecode/memory"
	"github.com/tencent/goom/internal/unexports2"
)

func TestMakeDumpFunc_PrintsArgsAndReturnsZero_PreGo124_AMD64(t *testing.T) {
	type Fn func(a int, b *int, c string, d float32) (int, *int)

	typ := reflect.TypeOf(Fn(nil))
	fAny := argdump.MakeDumpFunc(typ)
	f, ok := fAny.(Fn)
	if !ok {
		t.Fatalf("type assertion failed: got %T", fAny)
	}

	if os.Getenv("ARGDUMP_DEBUG") == "1" {
		callReflect, _ := unexports2.FindFuncByName("reflect.callReflect")
		callReflectAbi0, _ := unexports2.FindFuncByName("reflect.callReflect.abi0")
		t.Logf("DebugCallReflectPtr=0x%x callReflect=0x%x callReflect.abi0=0x%x", *argdump.DebugCallReflectPtr, callReflect, callReflectAbi0)
		if *argdump.DebugCallReflectPtr != 0 {
			t.Logf("patchedTargetBytes=%s", hex.EncodeToString(memory.RawRead(*argdump.DebugCallReflectPtr, 16)))
		}
		if callReflect != 0 {
			t.Logf("sym.callReflect.bytes=%s", hex.EncodeToString(memory.RawRead(callReflect, 16)))
		}
		if callReflectAbi0 != 0 {
			t.Logf("sym.callReflect.abi0.bytes=%s", hex.EncodeToString(memory.RawRead(callReflectAbi0, 16)))
		}
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
	if !strings.Contains(out, "arg0=") || !strings.Contains(out, "arg1=") ||
		!strings.Contains(out, "arg2=") || !strings.Contains(out, "arg3=") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

// Keep Makefile / compatibility tests working: they run -run=^TestUnit.
func TestUnitMakeDumpFunc_PrintsArgsAndReturnsZero_PreGo124_AMD64(t *testing.T) {
	TestMakeDumpFunc_PrintsArgsAndReturnsZero_PreGo124_AMD64(t)
}

