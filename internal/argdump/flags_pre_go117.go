//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package argdump

// DebugEnabled enables verbose diagnostics (stderr).
var DebugEnabled bool

// DumpEnabled controls whether argdump prints/encodes arguments/returns/panics.
var DumpEnabled = true


