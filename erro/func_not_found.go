package erro

import (
	"strings"

	"github.com/tencent/goom/internal/logger"
)

const prefix = "func not found: "

// FuncNotFoundError 函数未找到异常
type FuncNotFoundError struct {
	funcName    string
	suggestions []string
}

// Error 返回错误字符串
func (e *FuncNotFoundError) Error() string {
	msg := prefix + e.funcName
	if e.suggestions == nil {
		return msg
	}

	var noEmptyStrings []string
	for _, v := range e.suggestions {
		if v == "" {
			continue
		}
		noEmptyStrings = append(noEmptyStrings, v)
	}
	if len(noEmptyStrings) == 0 {
		return msg
	}

	tips := "\ndo you mean: (?) \n* "
	tips += strings.Join(noEmptyStrings, "\n* ")
	return msg + logger.Magenta.AddAll(tips)
}

// NewFuncNotFoundError 函数未找到
// funcName 函数名称
func NewFuncNotFoundError(funcName string) error {
	return &FuncNotFoundError{funcName: funcName}
}

// NewFuncNotFoundErrorWithSuggestion 函数未找到并给出提示
// funcName 函数名称
func NewFuncNotFoundErrorWithSuggestion(funcName string, suggestions []string) error {
	return &FuncNotFoundError{funcName: funcName, suggestions: suggestions}
}
