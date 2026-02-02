//go:build go1.17 && !go1.18 && arm64
// +build go1.17,!go1.18,arm64

package argdump_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
	"github.com/tencent/goom/internal/argdump/testtargets"
)

func TestPatchFunc_PreGo124_PrintsArgsAndPreservesReturn(t *testing.T) {
	// This is a legacy smoke test kept for go1.17. The full go1.24 test suite has been
	// ported into `patchfunc_go117_arm64_ported_test.go`, so running this again is redundant
	// and can exhaust small executable scratch regions on some platforms.
	t.Skip("covered by ported PatchFunc tests in patchfunc_go117_arm64_ported_test.go")
	// if os.Getenv("ARGDUMP_ENABLE_PATCH_TEST") != "1" {
	// 	t.Skip("set ARGDUMP_ENABLE_PATCH_TEST=1 to run PatchFunc tests (PatchFunc requires go1.24+)")
	// }
	prevDebug := argdump.DebugEnabled
	prevDump := argdump.DumpEnabled
	argdump.DebugEnabled = os.Getenv("ARGDUMP_DEBUG") == "1"
	argdump.DumpEnabled = true
	defer func() {
		argdump.DebugEnabled = prevDebug
		argdump.DumpEnabled = prevDump
	}()

	guard, err := argdump.PatchFunc(testtargets.CrossPkgAdd)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	// Capture stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fmt.Fprintln(os.Stderr, "[test] calling CrossPkgAdd")
	got := testtargets.CrossPkgAdd(10, 32)
	fmt.Fprintln(os.Stderr, "[test] returned from CrossPkgAdd got=", got)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	if got != 42 {
		t.Fatalf("want 42, got %d", got)
	}
	out := buf.String()
	if !strings.Contains(out, "arg0=10") || !strings.Contains(out, "arg1=32") || !strings.Contains(out, "ret0=42") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
	fmt.Fprintln(os.Stderr, "[test] stdout=", out)
}
