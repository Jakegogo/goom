// Package argdump provides function argument tracing capabilities for Go.
//
// It supports Go 1.13+ with architecture-specific implementations for:
//   - 386 (go1.13-go1.16)
//   - amd64 (go1.13-go1.16)
//   - arm64 (go1.13+)
//
// # Key APIs
//
//   - MakeDumpFunc: Creates a function that prints all arguments as JSON
//   - PatchFunc: Patches an existing function to trace its arguments
//
// # Implementation Notes
//
// The package handles multiple Go versions and ABIs:
//   - Pre-Go1.17: Stack-only ABI (ABI0)
//   - Go1.17+ arm64: Register-passing ABI with version-specific variations
//
// # Subpackages
//
//   - abi: ABI constants (platform/version-specific)
//   - internal/bitvec: Bitvector implementation for pointer tracking
//   - internal/mem: Memory utility functions
package argdump
