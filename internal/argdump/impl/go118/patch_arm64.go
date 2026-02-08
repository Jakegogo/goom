//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

package go118

// Re-export layer for arm64 patch functions.
// All implementation is in regabi package, see core_arm64.go for re-exports.
