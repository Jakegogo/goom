package erro

// TypeNotFoundError 类型没有找到
type TypeNotFoundError struct {
	typName string
}

// Error 返回错误字符串
func (t *TypeNotFoundError) Error() string {
	return "type not found: " + t.typName
}

// NewTypeNotFoundError 创建类型未找到异常
// typName 类型名称
func NewTypeNotFoundError(typName string) error {
	return &TypeNotFoundError{
		typName: typName,
	}
}
