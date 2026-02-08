package erro

// IllegalStatusError 状态错误异常
type IllegalStatusError struct {
	funcName string
	msg      string
}

// Error 返回错误字符串
func (i *IllegalStatusError) Error() string {
	return "Illegal status error when call " + i.funcName + " msg: " + i.msg
}

// NewIllegalStatusError 状态参数异常
// funcName 函数名
// msg 信息
func NewIllegalStatusError(funcName, msg string) error {
	return &IllegalStatusError{funcName: funcName, msg: msg}
}
