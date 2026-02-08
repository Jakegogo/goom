// Package regabi provides shared implementation for register-based ABI
// used in Go 1.17+ on amd64 and Go 1.18+ on arm64.
//
// This package consolidates the common code from go117, go118, and go124
// packages to eliminate duplication while maintaining full compatibility
// across different Go versions and architectures.
//
// The key design principles:
//  - Build tags control version-specific code compilation
//  - Type aliases handle interface{} vs any differences
//  - Zero runtime overhead (compile-time selection)
//  - Preserves exact ABI layouts required by Go runtime
package regabi
