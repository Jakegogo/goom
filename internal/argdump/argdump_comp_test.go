// Package argdump_test 对 argdump 包的测试
// 当前文件实现了对 internal/argdump 的单测对于不同 go 版本的兼容性测试（自动下载对应 Go 版本）。
package argdump_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tencent/goom/test"
)

const argdumpTestEnv = "ARGDUMP_COMPATIBILITY_TEST"

// supportedArch reports whether internal/argdump is expected to work on this arch.
func supportedArch(goarch string) bool {
	switch goarch {
	case "amd64", "arm64", "386":
		return true
	default:
		return false
	}
}

func minGoVersionFor(goos, goarch string) string {
	// There is no official darwin/arm64 binary release before Go 1.16.
	if goos == "darwin" && goarch == "arm64" {
		return "go1.16"
	}
	// Default to go1.13 for other platforms.
	return "go1.13.15"
}

func candidateVersions(goos, goarch string) []string {
	// Keep this list conservative: test.Run uses log.Fatalf on download failure.
	// Only include versions that are expected to have binary releases for the current platform.
	//
	// Note: update this list as needed when new Go versions are released.
	if goos == "darwin" && goarch == "arm64" {
		// Apple Silicon: start at go1.16.
		return []string{
			"go1.16",
			"go1.17.13",
			"go1.18.10",
			"go1.19.13",
			"go1.20.14",
			"go1.21.13",
			"go1.22.12",
			"go1.23.1",
			"go1.24.13",
			"go1.25.7",
		}
	}
	// Other platforms: include go1.13+.
	return []string{
		"go1.13.15",
		"go1.14.15",
		"go1.15.15",
		"go1.16",
		"go1.17.13",
		"go1.18.10",
		"go1.19.13",
		"go1.20.14",
		"go1.21.13",
		"go1.22.12",
		"go1.23.1",
		"go1.24.13",
		"go1.25.7",
	}
}

func goMajorMinor(version string) (string, bool) {
	// Accept "go1.16" and "go1.21.13" etc.
	if !strings.HasPrefix(version, "go") {
		return "", false
	}
	v := strings.TrimPrefix(version, "go")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return "", false
	}
	return strings.Join(parts[0:2], "."), true
}

func hasDownloadedToolchain(version string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	// Must match test.goroot() + test.unpackedOkay.
	root := filepath.Join(home, "sdk", version)
	_, err = os.Stat(filepath.Join(root, ".unpacked-success"))
	return err == nil
}

func mustHaveTimeForDownload(t *testing.T, version string) bool {
	t.Helper()
	type deadlineTester interface {
		Deadline() (time.Time, bool)
	}
	dt, ok := interface{}(t).(deadlineTester)
	if !ok {
		return true
	}
	dl, ok := dt.Deadline()
	if !ok {
		return true
	}
	// If the toolchain is already present, we don't need much time.
	if hasDownloadedToolchain(version) {
		return true
	}
	// Downloads can be large; require a healthy buffer.
	const minRemaining = 5 * time.Minute
	remain := time.Until(dl)
	if remain < minRemaining {
		t.Skipf("not enough time to download %s (remaining %s). Re-run with a larger -timeout (e.g. -timeout=30m).",
			version, remain.Round(time.Second))
		return false
	}
	return true
}

// TestArgdumpCompatibility checks internal/argdump compatibility across multiple Go versions.
// It auto-downloads toolchains using github.com/tencent/goom/test.Run.
func TestArgdumpCompatibility(t *testing.T) {
	if os.Getenv(argdumpTestEnv) == "true" {
		return
	}
	if !supportedArch(runtime.GOARCH) {
		t.Skipf("argdump compatibility test skipped: unsupported arch %s on %s", runtime.GOARCH, runtime.GOOS)
	}

	os.Setenv(argdumpTestEnv, "true")

	minV := minGoVersionFor(runtime.GOOS, runtime.GOARCH)
	versions := candidateVersions(runtime.GOOS, runtime.GOARCH)
	fmt.Printf("> [argdump] platform=%s/%s min=%s\n", runtime.GOOS, runtime.GOARCH, minV)

	for _, v := range versions {
		fmt.Printf("> [%s] start testing internal/argdump..\n", v)
		if !mustHaveTimeForDownload(t, v) {
			return
		}

		// Prepare toolchain (download if needed) and verify it runs.
		if err := test.Run(v, nil, "version"); err != nil {
			t.Errorf("[%s] env prepare fail: %v", v, err)
			break
		}

		logHandler := func(log string) {
			if strings.Contains(log, "--- FAIL:") {
				t.Errorf("[%s] run fail: see details in the log above.", v)
			}
			fmt.Println(log)
		}

		if vn, ok := goMajorMinor(v); ok {
			if err := test.Run(v, logHandler, "mod", "edit", "-go="+vn); err != nil {
				t.Errorf("[%s] mod edit error: %v, see details in the log above.", v, err)
				break
			}
		}

		// Run only argdump tests. PatchFunc tests are opt-in via ARGDUMP_ENABLE_PATCH_TEST.
		if err := test.Run(v, logHandler, "test", "-v", "-gcflags=all=-l", "-ldflags=-s=false", "-run=^TestUnit", "./"); err != nil {
			t.Errorf("[%s] run error: %v, see details in the log above.", v, err)
			break
		}

		if t.Failed() {
			break
		}
		t.Logf("[%s] internal/argdump compatibility ok.", v)
	}
}
