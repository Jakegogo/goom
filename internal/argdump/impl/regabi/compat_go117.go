//go:build go1.17 && !go1.18
// +build go1.17,!go1.18

package regabi

// anyCompat maps to interface{} in Go 1.17
// This allows us to write version-agnostic code that works
// with both interface{} (Go 1.17) and any (Go 1.18+)
type anyCompat = interface{}
