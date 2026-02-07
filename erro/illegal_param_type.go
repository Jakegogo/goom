package erro

import "fmt"

// IllegalParamType 参数类型错误异常
type IllegalParamType struct {
	paramName  string
	paramType  string
	expectType string
}

// Error 返回错误字符串
func (i *IllegalParamType) Error() string {
	return fmt.Sprintf("Illegal param type error, param: %s, type:%s, expect type: %s",
		i.paramName, i.paramType, i.expectType)
}

// NewIllegalParamTypeError 创建参数类型异常
// paramName 参数名
// paramType 参数类型
// expectType 期望类型
func NewIllegalParamTypeError(paramName, paramType, expectType string) error {
	return &IllegalParamType{paramName: paramName, paramType: paramType, expectType: expectType}
}
