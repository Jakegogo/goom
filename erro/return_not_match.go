package erro

// ReturnsNotMatch 返回参数不匹配异常
type ReturnsNotMatch struct {
	_ ArgsNotMatch
}

// Error 返回错误字符串
func (i *ReturnsNotMatch) Error() string {
	return i.Error()
}

// NewReturnsNotMatchError 创建参数异常
// funcDef 函数定义
// argLen 参数长度
// expectLen 期望长度
func NewReturnsNotMatchError(funcDef interface{}, argLen, expectLen int) error {
	return &ArgsNotMatch{funcDef: funcDef, argLen: argLen, expectLen: expectLen, typ: "returns"}
}
