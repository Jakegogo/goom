package arg

import (
	"fmt"
	"reflect"
)

// Expr 表达式接口, 实现了 equals、any、in、field(x)等表达式匹配
// 一般类型参数默认使用 equals 表达式, 实现了 Expr 接口的参数表达式将使用对于的表达式进行匹配参数
type Expr interface {
	// Eval 执行一个表达式,
	// 一般地, 如果执行结果是 true, 则参数 Match 成功
	// input 表达式执行时的入参
	Eval(input []reflect.Value, isVariadic bool) (bool, error)
	// Resolve 解析参数类型
	Resolve(types []reflect.Type, isVariadic bool) error
}

// AnyExpr 和任意参数值比较
type AnyExpr struct {
}

// Resolve AnyExpr 表达式解析
func (a *AnyExpr) Resolve(_ []reflect.Type, isVariadic bool) error {
	return nil
}

// Eval 执行 AnyExpr 表达式
func (a *AnyExpr) Eval(_ []reflect.Value, isVariadic bool) (bool, error) {
	return true, nil
}

// EqualsExpr 表达式实现了两个参数是否相等的规则计算
type EqualsExpr struct {
	arg  interface{}
	argV reflect.Value
}

// Resolve EqualsExpr 表达式解析
func (e *EqualsExpr) Resolve(types []reflect.Type, isVariadic bool) error {
	// types 只会有一个元素
	if len(types) != 1 {
		return fmt.Errorf("EqualsExpr.Resolve status error")
	}
	var err error
	e.argV, err = toValue(e.arg, types[0], isVariadic)
	return err
}

// Eval 执行 EqualsExpr 表达式
func (e *EqualsExpr) Eval(input []reflect.Value, isVariadic bool) (bool, error) {
	// input 只会有一个元素
	if len(input) != 1 {
		return false, fmt.Errorf("EqualsExpr.Resolve status error")
	}
	if equal(e.argV, input[0]) {
		return true, nil
	}
	return false, nil
}

// InExpr 包含表达式执行
type InExpr struct {
	args        []interface{}
	expressions [][]Expr
}

// Resolve InExpr 表达式解析
func (in *InExpr) Resolve(types []reflect.Type, isVariadic bool) error {
	expressions := make([][]Expr, 0, len(in.args))
	lastVariadicIndex := len(types) - 1
	for i, v := range in.args {
		param, ok := v.([]interface{})
		switch {
		case ok:
		case isVariadic && i >= lastVariadicIndex:
			// 可变参数需要展开参数数组, 为每个参数元素生成独立表达式
			rv := reflect.ValueOf(v)
			expandArgs := make([]interface{}, rv.Len())
			for j := 0; j < rv.Len(); j++ {
				expandArgs[j] = rv.Index(j).Interface()
			}
			param = expandArgs
		default:
			param = []interface{}{v}
		}

		expr, err := ToExpr(param, types, isVariadic)
		if err != nil {
			return err
		}
		expressions = append(expressions, expr)
	}
	in.expressions = expressions
	return nil
}

// Eval InExpr 表达式执行
func (in *InExpr) Eval(input []reflect.Value, isVariadic bool) (bool, error) {
	if isVariadic {
		// 可变参数需要展开参数数组
		expandArgs := make([]reflect.Value, 0, len(input))
		for _, v := range input {
			for i := 0; i < v.Len(); i++ {
				expandArgs = append(expandArgs, v.Index(i))
			}
		}
		input = expandArgs
	}
	for _, one := range in.expressions {
		if len(input) != len(one) {
			return false, nil
		}
		matched := true
		for i, param := range one {
			v, err := param.Eval([]reflect.Value{input[i]}, isVariadic)
			if err != nil {
				return false, err
			}
			if !v {
				matched = false
				break
			}
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
