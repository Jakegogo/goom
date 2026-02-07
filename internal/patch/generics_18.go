//go:build go1.18 && !go1.20
// +build go1.18,!go1.20

package patch

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	nameMatch = "\\.func(\\d)*(\\.\\d+)*$"
	nameReg   = regexp.MustCompile(nameMatch)
)

func init() {
	// 解析正则表达式，如果成功返回解释器
	if nameReg == nil {
		fmt.Println("regexp err")
		return
	}
}

// IsGenericsFunc 是否为泛型函数
func IsGenericsFunc(name string) bool {
	if isGenericsFunc20(name) {
		return true
	}
	result := nameReg.FindString(name)
	return len(result) > 0
}

// isGenericsFunc20 是否为泛型函数,同时兼容20版本的命名格式
func isGenericsFunc20(name string) bool {
	return strings.Contains(name, "[...]")
}
