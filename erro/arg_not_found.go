package erro

import "strconv"

// ArgNotFoundError 参数未找到异常
type ArgNotFoundError struct {
	funcName string
	arg      int
}

// Error 返回错误字符串
func (e *ArgNotFoundError) Error() string {
	return "arg not found: " + e.funcName + ":" + strconv.Itoa(e.arg)
}

// NewArgNotFoundError 函数未找到
// funcName 函数名称
// index 参数下标
func NewArgNotFoundError(funcName string, index int) error {
	return &ArgNotFoundError{funcName: funcName, arg: index}
}
