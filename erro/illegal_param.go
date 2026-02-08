package erro

// IllegalParamError 参数错误异常
type IllegalParamError struct {
	cause      error
	paramName  string
	paramValue string
	funcName   string
}

// Error 返回错误字符串
func (i *IllegalParamError) Error() (s string) {
	defer func() {
		if i.cause != nil {
			s = s + "\ncause: " + i.cause.Error()
		}
	}()
	if len(i.funcName) > 0 {
		return "Illegal param error when call" + i.funcName + ", param=" + i.paramName + ", value=" + i.paramValue
	}
	return "Illegal param error, param=" + i.paramName + ", value=" + i.paramValue
}

// Cause 获取错误的原因
func (i *IllegalParamError) Cause() error {
	return i.cause
}

// NewIllegalParamError 创建参数异常
// paramName 参数名
// paramValue 参数值
func NewIllegalParamError(paramName string, paramValue string) error {
	return &IllegalParamError{paramName: paramName, paramValue: paramValue}
}

// NewIllegalParamCError 创建参数异常
// paramName 参数名
// paramValue 参数值
func NewIllegalParamCError(paramName, paramValue string, cause error) error {
	return &IllegalParamError{paramName: paramName, paramValue: paramValue, cause: cause}
}

// NewIllegalCallError 创建参数异常
// funcName 函数名
// paramName 参数名
// paramValue 参数值
func NewIllegalCallError(funcName, paramName string, paramValue string) error {
	return &IllegalParamError{funcName: funcName, paramName: paramName, paramValue: paramValue}
}
