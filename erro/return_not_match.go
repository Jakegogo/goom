package erro

// ReturnsNotMatchError 返回参数不匹配异常
type ReturnsNotMatchError struct {
	_ ArgsNotMatchError
}

// Error 返回错误字符串
func (i *ReturnsNotMatchError) Error() string {
	return i.Error()
}

// NewReturnsNotMatchError 创建参数异常
// funcDef 函数定义
// argLen 参数长度
// expectLen 期望长度
func NewReturnsNotMatchError(funcDef interface{}, argLen, expectLen int) error {
	return &ArgsNotMatchError{funcDef: funcDef, argLen: argLen, expectLen: expectLen, typ: "returns"}
}
