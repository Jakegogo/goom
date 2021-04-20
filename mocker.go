// Package mocker 定义了mock的外层用户使用API定义,
// 包括函数、方法、接口、未导出函数(或方法的)的Mocker的实现。
// 当前文件定义了函数、方法、未导出函数(或方法)的Mocker的行为。
package mocker

import (
	"fmt"
	"reflect"
	"strings"

	"git.code.oa.com/goom/mocker/errobj"
	"git.code.oa.com/goom/mocker/internal/proxy"
	"git.code.oa.com/goom/mocker/internal/unexports"
)

// Mocker mock接口, 所有类型(函数、方法、未导出函数、接口等)的Mocker的抽象
type Mocker interface {
	// Apply 代理方法实现
	// 注意: Apply会覆盖之前设定的When条件和Return
	// 注意: 不支持在多个协程中并发地Apply不同的imp函数
	Apply(imp interface{})
	// Cancel 取消代理
	Cancel()
	// Canceled 是否已经被取消
	Canceled() bool
}

// ExportedMocker 导出函数mock接口
type ExportedMocker interface {
	Mocker
	// When 指定条件匹配
	When(args ...interface{}) *When
	// Return 执行返回值
	Return(args ...interface{}) *When
	// Origin 指定Mock之后的原函数, origin签名和mock的函数一致
	Origin(origin interface{}) ExportedMocker
	// If 条件表达式匹配
	If() *If
}

// UnExportedMocker 未导出函数mock接口
type UnExportedMocker interface {
	Mocker
	// As 将未导出函数(或方法)转换为导出函数(或方法)
	// As调用之后,请使用Return或When API的方式来指定mock返回。
	As(funcDef interface{}) ExportedMocker
	// Origin 指定Mock之后的原函数, origin签名和mock的函数一致
	Origin(origin interface{}) UnExportedMocker
}

// baseMocker mocker基础类型
type baseMocker struct {
	pkgName string
	origin  interface{}
	guard   MockGuard
	imp     interface{}

	when *When
	_if  *If
	// canceled 是否被取消
	canceled bool
}

// newBaseMocker 新增基础类型mocker
func newBaseMocker(pkgName string) *baseMocker {
	return &baseMocker{
		pkgName: pkgName,
	}
}

// applyByName 根据函数名称应用mock
func (m *baseMocker) applyByName(funcName string, imp interface{}) {
	guard, err := proxy.StaticProxyByName(funcName, imp, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy func name error: %v", err))
	}

	m.guard = NewPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = imp
}

// applyByFunc 根据函数应用mock
func (m *baseMocker) applyByFunc(funcDef interface{}, imp interface{}) {
	guard, err := proxy.StaticProxyByFunc(funcDef, imp, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy func definition error: %v", err))
	}

	m.guard = NewPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = imp
}

// applyByMethod 根据函数名应用mock
func (m *baseMocker) applyByMethod(structDef interface{}, method string, imp interface{}) {
	guard, err := proxy.StaticProxyByMethod(reflect.TypeOf(structDef), method, imp, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy method error: %v", err))
	}

	m.guard = NewPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = imp
}

// applyByIFaceMethod 根据接口方法应用mock
func (m *baseMocker) applyByIFaceMethod(ctx *proxy.IContext, iFace interface{}, method string, imp interface{},
	implV proxy.PFunc) {

	impV := reflect.TypeOf(imp)
	if impV.In(0) != reflect.TypeOf(&IContext{}) {
		panic(errobj.NewIllegalParamTypeError("<first arg>", impV.In(0).Name(), "*IContext"))
	}

	err := proxy.MakeInterfaceImpl(iFace, ctx, method, imp, implV)
	if err != nil {
		panic(errobj.NewTraceableErrorf("interface mock apply error", err))
	}

	m.guard = NewIFaceMockGuard(ctx)
	m.guard.Apply()
	m.imp = imp
}

// whens 指定的返回值
func (m *baseMocker) whens(when *When) error {
	m.imp = reflect.MakeFunc(when.funcTyp, m.callback).Interface()
	m.when = when

	return nil
}

// if 指定的返回值 TODO 功能在适配中
// func (m *baseMocker) ifs(_if *If) error {
// 	m.imp = reflect.MakeFunc(_if.funcTyp, m.callback).Interface()
// 	m._if = _if
//
// 	return nil
// }

// callback 通用的MakeFunc callback
func (m *baseMocker) callback(args []reflect.Value) (results []reflect.Value) {
	if m.when != nil {
		results = m.when.invoke(args)

		if results != nil {
			return results
		}
	}

	if m._if != nil {
		results = m._if.invoke(args)
		if results != nil {
			return results
		}
	}

	panic("not match any args, please spec default return use: mocker.Return()")
}

// Cancel 取消Mock
func (m *baseMocker) Cancel() {
	if m.guard != nil {
		m.guard.Cancel()
	}

	m.when = nil
	m._if = nil
	m.origin = nil
	m.canceled = true
}

// Canceled 是否被取消
func (m *baseMocker) Canceled() bool {
	return m.canceled
}

// MethodMocker 对结构体函数或方法进行mock
// 能支持到私有函数、私有类型的方法的Mock
type MethodMocker struct {
	*baseMocker
	structDef interface{}
	method    string
	methodIns interface{}
}

// NewMethodMocker 创建MethodMocker
// pkgName 包路径
// structDef 结构体变量定义, 不能为nil
func NewMethodMocker(pkgName string, structDef interface{}) *MethodMocker {
	return &MethodMocker{
		baseMocker: newBaseMocker(pkgName),
		structDef:  structDef,
	}
}

// Method 设置结构体的方法名
func (m *MethodMocker) Method(name string) ExportedMocker {
	if name == "" {
		panic("method is empty")
	}

	m.method = name
	sTyp := reflect.TypeOf(m.structDef)

	method, ok := sTyp.MethodByName(m.method)
	if !ok {
		panic("method " + m.method + " not found on " + sTyp.String())
	}

	m.methodIns = method.Func.Interface()

	return m
}

// ExportMethod 导出私有方法
func (m *MethodMocker) ExportMethod(name string) UnExportedMocker {
	if name == "" {
		panic("method is empty")
	}

	// 转换结构体名
	structName := typeName(m.structDef)
	if strings.Contains(structName, "*") {
		structName = fmt.Sprintf("(%s)", structName)
	}

	return (&UnexportedMethodMocker{
		baseMocker: m.baseMocker,
		structName: structName,
		methodName: name,
	}).Method(name)
}

// Apply 指定mock执行的回调函数
// mock回调函数, 需要和mock模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *MethodMocker) Apply(imp interface{}) {
	if m.method == "" {
		panic("method is empty")
	}

	m.applyByMethod(m.structDef, m.method, imp)
}

// When 指定条件匹配
func (m *MethodMocker) When(args ...interface{}) *When {
	if m.method == "" {
		panic("method is empty")
	}

	if m.when != nil {
		return m.when.When(args...)
	}

	sTyp := reflect.TypeOf(m.structDef)
	methodIns, ok := sTyp.MethodByName(m.method)
	if !ok {
		panic("method " + m.method + " not found on " + sTyp.String())
	}

	var (
		when *When
		err  error
	)

	if when, err = CreateWhen(m, methodIns.Func.Interface(), args, nil, true); err != nil {
		panic(err)
	}

	if err := m.whens(when); err != nil {
		panic(err)
	}

	m.Apply(m.imp)

	return when
}

// Return 指定返回值
func (m *MethodMocker) Return(returns ...interface{}) *When {
	if m.method == "" {
		panic("method is empty")
	}

	if m.when != nil {
		return m.when.Return(returns...)
	}

	var (
		when *When
		err  error
	)

	if when, err = CreateWhen(m, m.methodIns, nil, returns, true); err != nil {
		panic(err)
	}

	if err := m.whens(when); err != nil {
		panic(err)
	}

	m.Apply(m.imp)

	return when
}

// Origin 指定调用的原函数
func (m *MethodMocker) Origin(origin interface{}) ExportedMocker {
	m.origin = origin
	return m
}

// If 条件子句
func (m *MethodMocker) If() *If {
	return nil
}

// UnexportedMethodMocker 对结构体函数或方法进行mock
// 能支持到未导出类型、未导出类型的方法的Mock
type UnexportedMethodMocker struct {
	*baseMocker
	structName string
	methodName string
}

// NewUnexportedMethodMocker 创建未导出方法Mocker
// pkgName 包路径
// structName 结构体名称
func NewUnexportedMethodMocker(pkgName string, structName string) *UnexportedMethodMocker {
	return &UnexportedMethodMocker{
		baseMocker: newBaseMocker(pkgName),
		structName: structName,
	}
}

// objName 获取对象名
func (m *UnexportedMethodMocker) objName() string {
	return fmt.Sprintf("%s.%s.%s", m.pkgName, m.structName, m.methodName)
}

// Method 设置结构体的方法名
func (m *UnexportedMethodMocker) Method(name string) UnExportedMocker {
	m.methodName = name
	return m
}

// Apply 指定mock执行的回调函数
// mock回调函数, 需要和mock模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *UnexportedMethodMocker) Apply(imp interface{}) {
	name := m.objName()
	if name == "" {
		panic("method name is empty")
	}

	if !strings.Contains(name, "*") {
		_, _ = unexports.FindFuncByName(name)
	}

	m.applyByName(name, imp)
}

// Origin 调用原函数
func (m *UnexportedMethodMocker) Origin(origin interface{}) UnExportedMocker {
	m.origin = origin

	return m
}

// As 将未导出函数(或方法)转换为导出函数(或方法)
func (m *UnexportedMethodMocker) As(funcDef interface{}) ExportedMocker {
	name := m.objName()
	if name == "" {
		panic("method name is empty")
	}

	var (
		err           error
		originFuncPtr uintptr
	)

	originFuncPtr, err = unexports.FindFuncByName(name)
	if err != nil {
		panic(err)
	}

	newFunc := unexports.NewFuncWithCodePtr(reflect.TypeOf(funcDef), originFuncPtr)

	return &DefMocker{
		baseMocker: m.baseMocker,
		funcDef:    newFunc.Interface(),
	}
}

// UnexportedFuncMocker 对函数或方法进行mock
// 能支持到私有函数、私有类型的方法的Mock
type UnexportedFuncMocker struct {
	*baseMocker
	funcName string
}

// NewUnexportedFuncMocker 创建未导出函数Mocker
// pkgName 包路径
// funcName 函数名称
func NewUnexportedFuncMocker(pkgName, funcName string) *UnexportedFuncMocker {
	return &UnexportedFuncMocker{
		baseMocker: newBaseMocker(pkgName),
		funcName:   funcName,
	}
}

// objName 获取对象名
func (m *UnexportedFuncMocker) objName() string {
	return fmt.Sprintf("%s.%s", m.pkgName, m.funcName)
}

// Apply 指定mock执行的回调函数
// mock回调函数, 需要和mock模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *UnexportedFuncMocker) Apply(imp interface{}) {
	name := m.objName()

	m.applyByName(name, imp)
}

// Origin 调用原函数
func (m *UnexportedFuncMocker) Origin(origin interface{}) UnExportedMocker {
	m.origin = origin

	return m
}

// As 将未导出函数(或方法)转换为导出函数(或方法)
func (m *UnexportedFuncMocker) As(funcDef interface{}) ExportedMocker {
	name := m.objName()

	originFuncPtr, err := unexports.FindFuncByName(name)
	if err != nil {
		panic(err)
	}

	newFunc := unexports.NewFuncWithCodePtr(reflect.TypeOf(funcDef), originFuncPtr)

	return &DefMocker{
		baseMocker: m.baseMocker,
		funcDef:    newFunc.Interface(),
	}
}

// DefMocker 对函数或方法进行mock，使用函数定义筛选
type DefMocker struct {
	*baseMocker
	funcDef interface{}
}

// NewDefMocker 创建DefMocker
// pkgName 包路径
// funcDef 函数变量定义
func NewDefMocker(pkgName string, funcDef interface{}) *DefMocker {
	return &DefMocker{
		baseMocker: newBaseMocker(pkgName),
		funcDef:    funcDef,
	}
}

// Apply 代理方法实现
func (m *DefMocker) Apply(imp interface{}) {
	if m.funcDef == nil {
		panic("funcDef is empty")
	}

	var funcName = functionName(m.funcDef)

	if strings.HasSuffix(funcName, "-fm") {
		m.applyByName(strings.TrimRight(funcName, "-fm"), imp)
	} else {
		m.applyByFunc(m.funcDef, imp)
	}
}

// When 指定条件匹配
func (m *DefMocker) When(args ...interface{}) *When {
	if m.when != nil {
		return m.when.When(args...)
	}

	var (
		when *When
		err  error
	)

	if when, err = CreateWhen(m, m.funcDef, args, nil, false); err != nil {
		panic(err)
	}

	if err := m.whens(when); err != nil {
		panic(err)
	}

	m.Apply(m.imp)

	return when
}

// Return 代理方法返回
func (m *DefMocker) Return(returns ...interface{}) *When {
	if m.when != nil {
		return m.when.Return(returns...)
	}

	var (
		when *When
		err  error
	)

	if when, err = CreateWhen(m, m.funcDef, nil, returns, false); err != nil {
		panic(err)
	}

	if err := m.whens(when); err != nil {
		panic(err)
	}

	m.Apply(m.imp)

	return when
}

// Origin 调用原函数
func (m *DefMocker) Origin(origin interface{}) ExportedMocker {
	m.origin = origin
	return m
}

// If 条件子句
func (m *DefMocker) If() *If {
	return nil
}
