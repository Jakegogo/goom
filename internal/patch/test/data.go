// Package test 被测对象都放在这个包
package test

import "fmt"

var toggle = false

// No 返回 false 的函数
//
//go:noinline
func No() bool {
	if toggle {
		fmt.Println("false")
	}
	return false
}

// Yes 返回 true 的函数
//
//go:noinline
func Yes() bool { return true }

// S 结构体
type S struct{}

// Yes 返回 true 的方法
func (s *S) Yes() bool { return true }

// F 结构体
type F struct{}

// No 返回 false 的方法
//
//go:noinline
func (f *F) No() bool {
	// Stability note (arm64 patching):
	// Our arm64 jump stubs can be up to 24 bytes. If the target function/method is too small
	// (e.g. compiled into just "MOV x0,xzr; RET"), patch.Patch/InstanceMethod may return a nil guard
	// or fail to install safely, leading to nil deref when Apply() is called.
	//
	// Keep this method non-trivial so the entry has enough bytes to patch.
	// Keep this method large enough so arm64 patch stubs (up to 24 bytes) can be installed.
	// Also avoid being optimized into a tiny MOV+RET sequence.
	if toggle {
		fmt.Println("false")
	}
	return false
}
