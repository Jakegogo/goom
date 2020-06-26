package mocker

import (
	"git.code.oa.com/goom/mocker/errortype"
	"reflect"
	"strconv"
)

// When Mock条件匹配
type When struct {
	ExportedMocker
	returns []Return
	defaultReturns []interface{}
	funTyp reflect.Type
	// curArgs 当前指定的参数
	curArgs []interface{}
}

// CreateWhen 构造条件判断
// args 参数条件
// defaultReturns 默认返回值
func CreateWhen(m ExportedMocker, funcDef interface{}, args []interface{}, defaultReturns []interface{}) (*When, error) {
	impTyp := reflect.TypeOf(funcDef)
	if defaultReturns != nil && len(defaultReturns) < impTyp.NumOut() {
		return nil, errortype.NewIllegalParamError("returns:" + strconv.Itoa(len(defaultReturns) + 1), "'empty'")
	}
	if args != nil && len(args) < impTyp.NumIn() {
		return nil, errortype.NewIllegalParamError("args:" + strconv.Itoa(len(args) + 1), "'empty'")
	}
	return &When{
		ExportedMocker: m,
		defaultReturns: defaultReturns,
		funTyp:         impTyp,
		curArgs:		args,
	}, nil
}

// When当参数符合一定的条件
func (w *When) When(args ...interface{}) *When {
	w.curArgs = args
	return w
}

// Return 指定返回值
func (w *When) Return(args ...interface{}) *When {
	// TODO 归档到returns
	return w
}

// Whens 多个条件匹配
func (w *When) Whens(argsmap map[interface{}]interface{}) *When {
	return w
}

func(m *When) invoke(args1 []reflect.Value) (results []reflect.Value) {
	if len(m.returns) != 0 {
		// TODO 支持条件判断
		return results
	}

	// 使用默认参数
	for i, r := range m.defaultReturns {
		v := reflect.ValueOf(r)
		if r == nil &&
			(m.funTyp.Out(i).Kind() == reflect.Interface || m.funTyp.Out(i).Kind() == reflect.Ptr) {
			v = reflect.Zero(reflect.SliceOf(m.funTyp.Out(i)).Elem())
		} else if r != nil && m.funTyp.Out(i).Kind() == reflect.Interface {
			ptr := reflect.New(m.funTyp.Out(i))
			ptr.Elem().Set(v)
			v = ptr.Elem()
		}
		results = append(results, v)
	}
	return results
}

type Return struct {
}
