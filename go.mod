module github.com/tencent/goom

// Keep the module go version at the minimum supported toolchain so older Go
// toolchains (e.g. go1.17–go1.24) can run package tests without failing fast
// with "module requires Go X".
//
// Feature/ABI differences are handled via build tags in code, not via the go.mod directive.
go 1.18

require github.com/stretchr/testify v1.4.0
