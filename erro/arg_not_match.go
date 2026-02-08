package erro

import (
	"reflect"
	"strconv"
)

// ArgsNotMatchError 参数不匹配异常
type ArgsNotMatchError struct {
	funcDef   interface{}
	argLen    int
	expectLen int
	typ       string
}

// Error 返回错误字符串
func (i *ArgsNotMatchError) Error() string {
	if i.funcDef != nil {
		return i.typ + " length not match of func " + reflect.ValueOf(i.funcDef).String() +
			": " + strconv.Itoa(i.argLen) + ", expect: " + strconv.Itoa(i.expectLen)
	}

	return i.typ + "length not match: " + strconv.Itoa(i.argLen) + ", expect: " + strconv.Itoa(i.expectLen)
}

// NewArgsNotMatchError 创建参数异常
// funcDef 函数定义
// argLen 参数长度
// expectLen 期望长度
func NewArgsNotMatchError(funcDef interface{}, argLen int, expectLen int) error {
	return &ArgsNotMatchError{funcDef: funcDef, argLen: argLen, expectLen: expectLen, typ: "args"}
}
