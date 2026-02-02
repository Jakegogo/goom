//go:build go1.17 && !go1.18 && arm64
// +build go1.17,!go1.18,arm64

package testtargets

// Deployment/stability note (go1.17/darwin/arm64):
// Patching a function defined in the same *_test.go file (or even the same package) can be unstable
// because the patched target may share a code page with currently executing test code.
// Since patching on Apple Silicon toggles page permissions (W^X), this can crash.
//
// Keeping the PatchFunc target in a separate package reduces that risk and makes the test deterministic.
//
//go:noinline
func CrossPkgAdd(a int, b int) int { return a + b }

// CrossPkgScalar is a stable cross-package PatchFunc target that covers
// mixed scalar argument/return types (int + float64).
//
//go:noinline
func CrossPkgScalar(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}
