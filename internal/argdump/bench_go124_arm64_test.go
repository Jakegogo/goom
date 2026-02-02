//go:build go1.24 && arm64
// +build go1.24,arm64

package argdump_test

import (
	"os"
	"testing"

	"github.com/tencent/goom/internal/argdump"
)

type benchInfo struct {
	name string
	age  int
	addr string
}

//go:noinline
func benchAdd(a int, b int, info benchInfo) int {
	return a + b + info.age
}

//go:noinline
func benchAddScalar(a int, b int) int {
	return a + b
}

//go:noinline
func benchMultiReturn(a int, f float64, s string) (int, float64, string) {
	return a * 2, f * 1.5, s + "!"
}

//go:noinline
func benchMultiScalar(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}

func Benchmark_Unpatched_Add(b *testing.B) {
	info := benchInfo{name: "John", age: 30, addr: "123 Main St"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchAdd(10, 32, info)
	}
}

func Benchmark_Patched_Add_DumpToDevNull(b *testing.B) {
	info := benchInfo{name: "John", age: 30, addr: "123 Main St"}

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	// Redirect stdout to devnull so fmt.Printf doesn't dominate with terminal I/O.
	old := os.Stdout
	os.Stdout = devNull
	defer func() { os.Stdout = old }()

	argdump.DebugEnabled = false
	argdump.DumpEnabled = true
	guard, err := argdump.PatchFunc(benchAdd)
	if err != nil {
		b.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchAdd(10, 32, info)
	}
}

func Benchmark_Patched_Add_NoDump(b *testing.B) {
	argdump.DebugEnabled = false
	argdump.DumpEnabled = false
	// NOTE: NoDump benchmarks intentionally use scalar-only signatures to isolate hook/jump overhead.
	guard, err := argdump.PatchFunc(benchAddScalar)
	if err != nil {
		b.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = benchAddScalar(10, 32)
	}
}

func Benchmark_Unpatched_MultiReturn(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = benchMultiReturn(7, 2.5, "ok")
	}
}

func Benchmark_Patched_MultiReturn_DumpToDevNull(b *testing.B) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		b.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()

	old := os.Stdout
	os.Stdout = devNull
	defer func() { os.Stdout = old }()

	argdump.DebugEnabled = false
	argdump.DumpEnabled = true
	guard, err := argdump.PatchFunc(benchMultiReturn)
	if err != nil {
		b.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = benchMultiReturn(7, 2.5, "ok")
	}
}

func Benchmark_Patched_MultiReturn_NoDump(b *testing.B) {
	argdump.DebugEnabled = false
	argdump.DumpEnabled = false
	// NOTE: NoDump benchmarks intentionally use scalar-only signatures to isolate hook/jump overhead.
	guard, err := argdump.PatchFunc(benchMultiScalar)
	if err != nil {
		b.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = benchMultiScalar(7, 2.5)
	}
}
