//go:build go1.18 && !go1.24 && amd64
// +build go1.18,!go1.24,amd64

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

// CrossPkgVariadic is a stable cross-package variadic PatchFunc target.
//
//go:noinline
func CrossPkgVariadic(prefix string, nums ...int) int {
	sum := len(prefix)
	for _, n := range nums {
		sum += n
	}
	return sum
}

// CrossPkgScalar2 is a second cross-package scalar target to avoid
// patching the same symbol multiple times in NoDump tests.
//
//go:noinline
func CrossPkgScalar2(a int, f float64) (int, float64) {
	return a * 3, f * 2
}

