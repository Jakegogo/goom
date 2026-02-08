package erro

import "strings"

// GcFlags 编译标记,取消函数内联
var GcFlags = NewRequiredBuildFlagsError("-gcflags=\"all=-l\"")

// LdFlags 编译标记,取消符号压缩
var LdFlags = NewRequiredBuildFlagsError("-ld=flags=\"-s=false\"")

// RequiredBuildFlagsError 编译标记未找到
// 典型的比如: -gcflags="all=-l", -ldflags="-s=false"
type RequiredBuildFlagsError struct {
	TraceableError
	flags []string
}

// NewRequiredBuildFlagsError 创建编译标记未找到异常
// flags 编译、链接标记
func NewRequiredBuildFlagsError(flags ...string) *RequiredBuildFlagsError {
	return &RequiredBuildFlagsError{
		flags: flags,
		TraceableError: TraceableError{
			errStr: "required build flags in your test command: " + flagsString(flags) + "\n" +
				"for example: go test -gcflags=\"all=-l\" -ldflags=\"-s=false\" ./...",
		},
	}
}

// flagsString 将标记数组转换为字符串
func flagsString(flags []string) string {
	return strings.Join(flags, ",")
}
