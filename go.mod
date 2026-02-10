module github.com/tencent/goom

// Keep the module go version at the minimum supported toolchain so older Go
// toolchains (e.g. go1.17–go1.24) can run package tests without failing fast
// with "module requires Go X".
//
// Feature/ABI differences are handled via build tags in code, not via the go.mod directive.
go 1.18

require github.com/stretchr/testify v1.11.1

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
