//go:build go1.17 && !go1.18 && amd64
// +build go1.17,!go1.18,amd64

package testtargets

//go:noinline
func CrossPkgAdd(a int, b int) int { return a + b }

// CrossPkgScalar is a stable cross-package PatchFunc target that covers
// mixed scalar argument/return types (int + float64).
//
//go:noinline
func CrossPkgScalar(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}

