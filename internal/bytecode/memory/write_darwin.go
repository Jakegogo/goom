//go:build darwin && (amd64 || arm64)
// +build darwin
// +build amd64 arm64

package memory

//go:cgo_import_dynamic mach_task_self mach_task_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic mach_vm_protect mach_vm_protect "/usr/lib/libSystem.B.dylib"
