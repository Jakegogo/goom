package erro

// FieldNotFoundError 类型没有找到
type FieldNotFoundError struct {
	typName   string
	fieldName string
}

// Error 返回错误字符串
func (t *FieldNotFoundError) Error() string {
	return "field not found: " + t.typName + "." + t.fieldName
}

// NewFieldNotFoundError 创建类型未找到异常
// typName 类型名称
// fieldName 属性名称
func NewFieldNotFoundError(typName string, fieldName string) error {
	return &FieldNotFoundError{typName: typName, fieldName: fieldName}
}
