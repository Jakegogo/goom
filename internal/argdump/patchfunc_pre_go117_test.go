//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tencent/goom/internal/argdump"
)

//go:noinline
func preGo117Add(a int, b int) int { return a + b }

func TestUnitPatchFunc_PreGo117_PrintsArgsAndPreservesReturn(t *testing.T) {
	// PatchFunc modifies executable code; keep it opt-in for CI stability.
	// Enable via: ARGDUMP_ENABLE_PATCH_TEST=1
	if os.Getenv("ARGDUMP_ENABLE_PATCH_TEST") != "1" {
		t.Skip("set ARGDUMP_ENABLE_PATCH_TEST=1 to run PatchFunc tests")
	}

	*argdump.DebugEnabled =os.Getenv("ARGDUMP_DEBUG") == "1"

	guard, err := argdump.PatchFunc(preGo117Add)
	if err != nil {
		t.Fatalf("PatchFunc: %v", err)
	}
	guard.Apply()
	defer guard.UnpatchWithLock()

	out := captureStdout(t, func() {
		got := preGo117Add(10, 32)
		if got != 42 {
			t.Fatalf("want 42, got %d", got)
		}
	})

	if !strings.Contains(out, "arg0=") || !strings.Contains(out, "arg1=") {
		t.Fatalf("unexpected stdout:\n%s", out)
	}
	if !strings.Contains(out, "ret0=") {
		t.Fatalf("expected return dump, got:\n%s", out)
	}
}
