//go:build go1.18 && !go1.24 && amd64
// +build go1.18,!go1.24,amd64

package argdump_test

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
	"github.com/tencent/goom/internal/argdump/testtargets"
)

type Info struct {
	name string
	age  int
	addr string
}

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

func captureStdoutRecover(t *testing.T, fn func()) (out string, recovered interface{}) {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Defer order matters: recover first, then drain pipe, then restore stdout.
	defer func() { os.Stdout = old }()
	defer func() {
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		out = buf.String()
	}()
	defer func() { recovered = recover() }()

	fn()
	return out, recovered
}

//go:noinline
func add(a int, b int, info Info) int {
	if a < 0 {
		print("a < 0")
	}
	return a + b
}

func TestPatchFunc_PreGo124_PrintsArgsAndPreservesReturn(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	guard, err := argdump.PatchFunc(add)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	var got int
	out := captureStdout(t, func() {
		got = add(10, 32, Info{name: "John", age: 30, addr: "123 Main St"})
	})

	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
	if !strings.Contains(out, "arg0=10") ||
		!strings.Contains(out, "arg1=32") ||
		!strings.Contains(out, `"name":"John"`) ||
		!strings.Contains(out, `"age":30`) ||
		!strings.Contains(out, `"addr":"123 Main St"`) ||
		!strings.Contains(out, "ret0=42") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

//go:noinline
func multiReturnFloat(a int, f float64, s string) (int, float64, string) {
	return a * 2, f * 1.5, s + "!"
}

func TestPatchFunc_PreGo124_MultiReturnAndFloat(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	guard, err := argdump.PatchFunc(multiReturnFloat)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	var r0 int
	var r1 float64
	var r2 string
	out := captureStdout(t, func() {
		r0, r1, r2 = multiReturnFloat(7, 2.5, "ok")
	})

	if r0 != 14 || r1 != 3.75 || r2 != "ok!" {
		t.Fatalf("unexpected returns: %d %g %q", r0, r1, r2)
	}
	if !strings.Contains(out, "arg0=7") ||
		!strings.Contains(out, "arg1=2.5") ||
		!strings.Contains(out, `arg2="ok"`) ||
		!strings.Contains(out, "ret0=14") ||
		!strings.Contains(out, "ret1=3.75") ||
		!strings.Contains(out, `ret2="ok!"`) {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

type Deep4 struct{ v int }
type Deep3 struct{ d Deep4 }
type Deep2 struct{ d Deep3 }
type Deep1 struct{ d Deep2 }

//go:noinline
func returnsDeep(x Deep1) Deep1 { return x }

func TestPatchFunc_PreGo124_MaxDepth3(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	guard, err := argdump.PatchFunc(returnsDeep)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	in := Deep1{d: Deep2{d: Deep3{d: Deep4{v: 123}}}}
	var outv Deep1
	out := captureStdout(t, func() {
		outv = returnsDeep(in)
	})
	_ = outv // value equality isn't the point here; we care about dump stability.
	// abijson writes strings via json.Marshal, which escapes '<'/'>' into \u003c/\u003e.
	if !strings.Contains(out, "<max_depth>") && !strings.Contains(out, `\\u003cmax_depth\\u003e`) && !strings.Contains(out, `\u003cmax_depth\u003e`) {
		t.Fatalf("expected max depth marker in stdout (MaxDepth=3), got:\n%s", out)
	}
}

//go:noinline
func variadicSum(prefix string, nums ...int) int {
	sum := len(prefix)
	for _, n := range nums {
		sum += n
	}
	return sum
}

func TestPatchFunc_PreGo124_Variadic(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	guard, err := argdump.PatchFunc(variadicSum)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	var got int
	out := captureStdout(t, func() {
		got = variadicSum("x", 1, 2, 3)
	})
	if got != 7 {
		t.Fatalf("want 7, got %d", got)
	}
	// variadic arg is materialized as a []int at the call boundary for the callee.
	if !strings.Contains(out, `arg0="x"`) ||
		!strings.Contains(out, "arg1=[1,2,3]") ||
		!strings.Contains(out, "ret0=7") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

//go:noinline
func willPanic(a int, msg string) int {
	if a > 0 {
		panic("boom:" + msg)
	}
	return a
}

func TestPatchFunc_PreGo124_PanicPropagatesAndRestores(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	guard, err := argdump.PatchFunc(willPanic)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	out, rec := captureStdoutRecover(t, func() {
		_ = willPanic(1, "x")
	})
	if *argdump.DebugEnabled {
		_, _ = os.Stderr.WriteString("out:\n" + out + "\n")
	}
	if rec == nil {
		t.Fatalf("expected panic, got nil")
	}
	if !strings.Contains(out, "arg0=1") || !strings.Contains(out, `arg1="x"`) || !strings.Contains(out, `panic="boom:x"`) {
		t.Fatalf("expected args dump before panic, got:\n%s", out)
	}

	// Ensure patch is restored even after panic: calling again should still go through dump path.
	out2, _ := captureStdoutRecover(t, func() {
		_ = willPanic(1, "y")
	})
	if *argdump.DebugEnabled {
		_, _ = os.Stderr.WriteString("out2:\n" + out2 + "\n")
	}
	if !strings.Contains(out2, `arg1="y"`) {
		t.Fatalf("expected patch to remain active after panic, got:\n%s", out2)
	}
}

func TestPatchFunc_PreGo124_Closure(t *testing.T) {
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	prevDump := *argdump.DumpEnabled
	*argdump.DumpEnabled = true
	defer func() { *argdump.DumpEnabled = prevDump }()

	base := 5
	// Captures `base` (closure).
	clos := func(a int) int { return a + base }

	guard, err := argdump.PatchFunc(clos)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	var got int
	out := captureStdout(t, func() {
		got = clos(7)
	})
	if got != 12 {
		t.Fatalf("want 12, got %d", got)
	}
	if !strings.Contains(out, "arg0=7") || !strings.Contains(out, "ret0=12") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

//go:noinline
func multiScalar1(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}

//go:noinline
func multiScalarNoDump(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}

func Test_Patched_MultiReturn_NoDump(t *testing.T) {
	// NOTE (important / historical):
	// We intentionally patch a target defined in THIS test file.
	//
	// In some go test builds, patching a function defined in a different *_test.go file can be unstable:
	// the "function pointer" we patch may not be the exact entry PC used by the call sites (wrappers /
	// different entrypoints after linking/layout). When that happens, the call still reaches our
	// reflect.callReflect hook, but the `ctxt` passed into callDump is not our dumpFuncImpl, causing:
	// - `magic==0` (or random)
	// - and then a crash in reflect (e.g. "funcLayout of non-func type") when delegating.
	//
	// Keeping the NoDump target in-file makes the entrypoint stable and keeps this test deterministic.
	prevDebug := *argdump.DebugEnabled
	prevDump := *argdump.DumpEnabled
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	*argdump.DumpEnabled = false
	defer func() {
		*argdump.DebugEnabled = prevDebug
		*argdump.DumpEnabled = prevDump
	}()

	fn := reflect.ValueOf(multiScalarNoDump)
	fn.Call([]reflect.Value{reflect.ValueOf(7), reflect.ValueOf(2.5)})
	runtime.KeepAlive(fn)

	// NOTE: NoDump benchmarks intentionally use scalar-only signatures to isolate hook/jump overhead.
	guard, err := argdump.PatchFunc(multiScalarNoDump)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	out := captureStdout(t, func() {
		_, _ = multiScalarNoDump(7, 2.5)
	})

	if out != "" {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

func Test_Patched_MultiReturn_NoDump2(t *testing.T) {
	prevDebug := *argdump.DebugEnabled
	prevDump := *argdump.DumpEnabled
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	*argdump.DumpEnabled = false
	defer func() {
		*argdump.DebugEnabled = prevDebug
		*argdump.DumpEnabled = prevDump
	}()

	fn := reflect.ValueOf(multiScalar1)
	fn.Call([]reflect.Value{reflect.ValueOf(7), reflect.ValueOf(2.5)})
	runtime.KeepAlive(fn)

	// NOTE: NoDump benchmarks intentionally use scalar-only signatures to isolate hook/jump overhead.
	guard, err := argdump.PatchFunc(multiScalar1)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	out := captureStdout(t, func() {
		_, _ = multiScalar1(7, 2.5)
	})

	if out != "" {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

func Test_Patched_CrossPackage_NoDump(t *testing.T) {
	prevDebug := *argdump.DebugEnabled
	prevDump := *argdump.DumpEnabled
	*argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	*argdump.DumpEnabled = false
	defer func() {
		*argdump.DebugEnabled = prevDebug
		*argdump.DumpEnabled = prevDump
	}()

	// Warm the symbol so the call site is present.
	fn := reflect.ValueOf(testtargets.CrossPkgScalar)
	fn.Call([]reflect.Value{reflect.ValueOf(7), reflect.ValueOf(2.5)})
	runtime.KeepAlive(fn)

	guard, err := argdump.PatchFunc(testtargets.CrossPkgScalar)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	var gotI int
	var gotF float64
	out := captureStdout(t, func() {
		gotI, gotF = testtargets.CrossPkgScalar(7, 2.5)
	})
	if gotI != 14 || gotF != 3.75 {
		t.Fatalf("unexpected return: got (%d, %v) want (14, 3.75)", gotI, gotF)
	}
	if out != "" {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
}

