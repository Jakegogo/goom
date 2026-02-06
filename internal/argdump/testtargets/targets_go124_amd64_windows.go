//go:build go1.24 && amd64 && windows
// +build go1.24,amd64,windows

package testtargets

// CrossPkgScalar is a stable cross-package patch target.
//
//go:noinline
func CrossPkgScalar(a int, f float64) (int, float64) {
	return a * 2, f * 1.5
}

