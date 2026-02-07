// Package mocker 定义了 mock 的外层用户使用 API 定义,
// 包括函数、方法、接口、未导出函数(或方法的)的 Mocker 的实现。
// 当前文件定义了函数、方法、未导出函数(或方法)的 Mocker 的行为。
package mocker

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/tencent/goom/erro"
	"github.com/tencent/goom/internal/iface"
	"github.com/tencent/goom/internal/logger"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/proxy"
	"github.com/tencent/goom/internal/unexports2"
)

// Mocker mock 接口, 所有类型(函数、方法、未导出函数、接口等)的 Mocker 的抽象
type Mocker interface {
	// Apply 代理方法实现
	// 注意: Apply 会覆盖之前设定的 When 条件和 Return
	// 注意: 不支持在多个协程中并发地 Apply 不同的 imp 函数
	Apply(callback interface{})
	// Cancel 取消代理
	Cancel()
	// Canceled 是否已经被取消
	Canceled() bool
	// String mock 的名称或描述, 方便调试和问题排查
	String() string
}

// ExportedMocker 导出函数 mock 接口
type ExportedMocker interface {
	Mocker
	// When 指定条件匹配
	When(specArg ...interface{}) *When
	// Return 执行返回值
	Return(value ...interface{}) *When
	// Returns 依次按顺序返回值, 如果是多参可使用[]interface{}
	Returns(values ...interface{}) *When
	// Origin 指定 Mock 之后的原函数, origin 签名和 mock 的函数一致
	Origin(originFunc interface{}) ExportedMocker
}

// UnExportedMocker 未导出函数 mock 接口
type UnExportedMocker interface {
	Mocker
	// As 将未导出函数(或方法)转换为导出函数(或方法)
	// As 调用之后,请使用 Return 或 When API 的方式来指定 mock 返回。
	As(aFunc interface{}) ExportedMocker
	// Origin 指定 Mock 之后的原函数, origin 签名和 mock 的函数一致
	Origin(originFunc interface{}) UnExportedMocker
}

// baseMocker mocker 基础类型
type baseMocker struct {
	pkgName string
	origin  interface{}
	guard   MockGuard
	funcDef interface{}
	imp     interface{}

	when *When
	// canceled 是否被取消
	canceled bool
}

// newBaseMocker 新增基础类型 mocker
func newBaseMocker(pkgName string) *baseMocker {
	return &baseMocker{
		pkgName: pkgName,
	}
}

// applyByName 根据函数名称应用 mock
func (m *baseMocker) applyByName(funcName string, callback interface{}) {
	guard, err := proxy.FuncName(funcName, callback, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy func name error: %v", err))
	}

	m.guard = newPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = callback
}

// applyByFunc 根据函数应用 mock
func (m *baseMocker) applyByFunc(funcDef, callback interface{}) {
	guard, err := proxy.Func(funcDef, callback, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy func definition error: %v", err))
	}

	m.guard = newPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = callback
	m.funcDef = funcDef
}

// applyByMethod 根据函数名应用 mock
func (m *baseMocker) applyByMethod(structDef interface{}, method string, callback interface{}) {
	guard, err := proxy.Method(reflect.TypeOf(structDef), method, callback, m.origin)
	if err != nil {
		panic(fmt.Sprintf("proxy method error: %v", err))
	}

	m.guard = newPatchMockGuard(guard)
	m.guard.Apply()
	m.imp = callback
	m.funcDef = reflect.ValueOf(structDef).MethodByName(method).Interface()
}

// applyByIFaceMethod 根据接口方法应用 mock
func (m *baseMocker) applyByIFaceMethod(ctx *iface.IContext, iFace interface{}, method string, callback interface{},
	implV iface.PFunc) {

	impV := reflect.TypeOf(callback)
	if impV.In(0) != reflect.TypeOf(&IContext{}) {
		panic(erro.NewIllegalParamTypeError("<first arg>", impV.In(0).Name(), "*IContext"))
	}

	err := proxy.Interface(iFace, ctx, method, callback, implV)
	if err != nil {
		panic(erro.NewTraceableErrorc("interface mock apply error", err))
	}

	m.guard = newIFaceMockGuard(ctx)
	m.guard.Apply()
	m.imp = callback
}

// whens 指定的返回值
func (m *baseMocker) whens(when *When) error {
	m.imp = reflect.MakeFunc(when.funcTyp, m.callback).Interface()
	m.when = when
	return nil
}

// callback 通用的 MakeFunc callback
func (m *baseMocker) callback(args []reflect.Value) (results []reflect.Value) {
	if m.canceled && m.funcDef != nil {
		return reflect.ValueOf(m.funcDef).Call(args)
	}
	if m.when != nil {
		results = m.when.invoke(args)
		if results != nil {
			return results
		}
	}
	panic("there is no suitable condition matched, or set default return with: mocker.Return(...)")
}

// Cancel 取消 Mock
func (m *baseMocker) Cancel() {
	if m.guard != nil {
		m.guard.Cancel()
	}
	m.when = nil
	m.origin = nil
	m.canceled = true
}

// Canceled 是否被取消
func (m *baseMocker) Canceled() bool {
	return m.canceled
}

// MethodMocker 对结构体函数或方法进行 mock
// 能支持到私有函数、私有类型的方法的 Mock
type MethodMocker struct {
	*baseMocker
	structDef interface{}
	method    string
	methodIns interface{}
}

// NewMethodMocker 创建 MethodMocker
// pkgName 包路径
// structDef 结构体变量定义, 不能为 nil
func NewMethodMocker(pkgName string, structDef interface{}) *MethodMocker {
	return &MethodMocker{
		baseMocker: newBaseMocker(pkgName),
		structDef:  structDef,
	}
}

// String mock 的名称或描述, 方便调试和问题排查
func (m *MethodMocker) String() string {
	t := reflect.TypeOf(m.structDef)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return fmt.Sprintf("%s.(%s).%s", t.PkgPath(), t.Name(), m.method)
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

	packageName := packageName(m.structDef)
	m.baseMocker.pkgName = packageName

	return (&UnexportedMethodMocker{
		baseMocker: m.baseMocker,
		structName: structName,
		methodName: name,
	}).Method(name)
}

// Apply 指定 mock 执行的回调函数
// mock 回调函数, 需要和 mock 模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *MethodMocker) Apply(callback interface{}) {
	m.doApply(callback)
}

func (m *MethodMocker) doApply(imp interface{}) {
	if m.method == "" {
		panic("method is empty")
	}
	imp, _ = interceptDebugInfo(imp, nil, m)
	m.applyByMethod(m.structDef, m.method, imp)
	logger.Consolefc(logger.DebugLevel, "mocker [%s] apply.", logger.Caller(6), m.String())
}

// When 指定条件匹配
func (m *MethodMocker) When(specArg ...interface{}) *When {
	if m.method == "" {
		panic("method is empty")
	}
	if m.when != nil {
		return m.when.When(specArg...)
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
	if when, err = CreateWhen(m, methodIns.Func.Interface(), specArg, nil, true); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}

	m.doApply(m.imp)
	return when
}

// Return 指定返回值
func (m *MethodMocker) Return(value ...interface{}) *When {
	if m.method == "" {
		panic("method is empty")
	}
	if m.when != nil {
		return m.when.Return(value...)
	}

	var (
		when *When
		err  error
	)
	if when, err = CreateWhen(m, m.methodIns, nil, value, true); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}
	m.doApply(m.imp)
	return when
}

// Returns 依次按顺序返回值
func (m *MethodMocker) Returns(values ...interface{}) *When {
	if m.method == "" {
		panic("method is empty")
	}
	if m.when != nil {
		return m.when.Returns(values...)
	}

	var (
		when *When
		err  error
	)
	if when, err = CreateWhen(m, m.methodIns, nil, nil, true); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}
	m.when.Returns(values...)
	m.doApply(m.imp)
	return when
}

// Origin 指定调用的原函数
func (m *MethodMocker) Origin(originFunc interface{}) ExportedMocker {
	m.origin = originFunc
	return m
}

// UnexportedMethodMocker 对结构体函数或方法进行 mock
// 能支持到未导出类型、未导出类型的方法的 Mock
type UnexportedMethodMocker struct {
	*baseMocker
	structName string
	methodName string
}

// NewUnexportedMethodMocker 创建未导出方法 Mocker
// pkgName 包路径
// structName 结构体名称
func NewUnexportedMethodMocker(pkgName, structName string) *UnexportedMethodMocker {
	return &UnexportedMethodMocker{
		baseMocker: newBaseMocker(pkgName),
		structName: structName,
	}
}

// String mock 的名称或描述, 方便调试和问题排查
func (m *UnexportedMethodMocker) String() string {
	return fmt.Sprintf("%s.%s.%s", m.pkgName, m.structName, m.methodName)
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

// Apply 指定 mock 执行的回调函数
// mock 回调函数, 需要和 mock 模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *UnexportedMethodMocker) Apply(callback interface{}) {
	name := m.objName()
	if name == "" {
		panic("method name is empty")
	}

	if !strings.Contains(name, "*") {
		_, _ = unexports2.FindFuncByName(name)
	}

	callback, _ = interceptDebugInfo(callback, nil, m)
	m.applyByName(name, callback)
	logger.Consolefc(logger.DebugLevel, "mocker [%s] apply.", logger.Caller(5), m.String())
}

// Origin 调用原函数
func (m *UnexportedMethodMocker) Origin(originFunc interface{}) UnExportedMocker {
	m.origin = originFunc
	return m
}

// As 将未导出函数(或方法)转换为导出函数(或方法)
func (m *UnexportedMethodMocker) As(aFunc interface{}) ExportedMocker {
	name := m.objName()
	if name == "" {
		panic("method name is empty")
	}

	var (
		err           error
		originFuncPtr uintptr
	)
	originFuncPtr, err = unexports2.FindFuncByName(name)
	if err != nil {
		panic(err)
	}
	newFunc := unexports2.NewFuncWithCodePtr(reflect.TypeOf(aFunc), originFuncPtr)
	return &DefMocker{
		baseMocker: m.baseMocker,
		funcDef:    newFunc.Interface(),
	}
}

// UnexportedFuncMocker 对函数或方法进行 mock
// 能支持到私有函数、私有类型的方法的 Mock
type UnexportedFuncMocker struct {
	*baseMocker
	funcName string
}

// NewUnexportedFuncMocker 创建未导出函数 Mocker
// pkgName 包路径
// funcName 函数名称
func NewUnexportedFuncMocker(pkgName, funcName string) *UnexportedFuncMocker {
	return &UnexportedFuncMocker{
		baseMocker: newBaseMocker(pkgName),
		funcName:   funcName,
	}
}

// String mock 的名称或描述, 方便调试和问题排查
func (m *UnexportedFuncMocker) String() string {
	return fmt.Sprintf("%s.%s", m.pkgName, m.funcName)
}

// objName 获取对象名
func (m *UnexportedFuncMocker) objName() string {
	return fmt.Sprintf("%s.%s", m.pkgName, m.funcName)
}

// Apply 指定 mock 执行的回调函数
// mock 回调函数, 需要和 mock 模板函数的签名保持一致
// 方法的参数签名写法比如: func(s *Struct, arg1, arg2 type), 其中第一个参数必须是接收体类型
func (m *UnexportedFuncMocker) Apply(callback interface{}) {
	callback, _ = interceptDebugInfo(callback, nil, m)
	m.applyByName(m.objName(), callback)
	logger.Consolefc(logger.DebugLevel, "mocker [%s] apply.", logger.Caller(5), m.String())
}

// Origin 调用原函数
func (m *UnexportedFuncMocker) Origin(originFunc interface{}) UnExportedMocker {
	m.origin = originFunc
	return m
}

// As 将未导出函数(或方法)转换为导出函数(或方法)
func (m *UnexportedFuncMocker) As(funcDef interface{}) ExportedMocker {
	originFuncPtr, err := unexports2.FindFuncByName(m.objName())
	if err != nil {
		panic(err)
	}

	newFunc := unexports2.NewFuncWithCodePtr(reflect.TypeOf(funcDef), originFuncPtr)
	return &DefMocker{
		baseMocker: m.baseMocker,
		funcDef:    newFunc.Interface(),
	}
}

// DefMocker 对函数或方法进行 mock，使用函数定义筛选
type DefMocker struct {
	*baseMocker
	funcDef interface{}
}

// String mock 的名称或描述
func (m *DefMocker) String() string {
	return runtime.FuncForPC(reflect.ValueOf(m.funcDef).Pointer()).Name()
}

// NewDefMocker 创建 DefMocker
// pkgName 包路径
// funcDef 函数变量定义
func NewDefMocker(pkgName string, funcDef interface{}) *DefMocker {
	return &DefMocker{
		baseMocker: newBaseMocker(pkgName),
		funcDef:    funcDef,
	}
}

// Apply 代理方法实现
func (m *DefMocker) Apply(callback interface{}) {
	m.doApply(callback)
}

func (m *DefMocker) doApply(imp interface{}) {
	if m.funcDef == nil {
		panic("funcDef is empty")
	}

	funcName := functionName(m.funcDef)
	imp, _ = interceptDebugInfo(imp, nil, m)
	if patch.IsGenericsFunc(funcName) {
		// for generic variants func
		m.applyByFunc(m.funcDef, imp)
	} else if strings.HasSuffix(funcName, "-fm") {
		// TODO 理清-fm的用意
		m.applyByName(strings.TrimSuffix(funcName, "-fm"), imp)
	} else {
		m.applyByFunc(m.funcDef, imp)
	}
	logger.Consolefc(logger.DebugLevel, "mocker [%s] apply.", logger.Caller(6), m.String())
}

// When 指定条件匹配
func (m *DefMocker) When(specArg ...interface{}) *When {
	if m.when != nil {
		return m.when.When(specArg...)
	}
	var (
		when *When
		err  error
	)
	if when, err = CreateWhen(m, m.funcDef, specArg, nil, false); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}
	m.doApply(m.imp)
	return when
}

// Return 代理方法返回
func (m *DefMocker) Return(value ...interface{}) *When {
	if m.when != nil {
		return m.when.Return(value...)
	}
	var (
		when *When
		err  error
	)
	if when, err = CreateWhen(m, m.funcDef, nil, value, false); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}
	m.doApply(m.imp)
	return when
}

// Returns 依次按顺序返回值, 如果是多参可使用[]interface{}
func (m *DefMocker) Returns(values ...interface{}) *When {
	if m.when != nil {
		return m.when.Returns(values...)
	}
	var (
		when *When
		err  error
	)
	if when, err = CreateWhen(m, m.funcDef, nil, nil, false); err != nil {
		panic(err)
	}
	if err := m.whens(when); err != nil {
		panic(err)
	}
	m.when.Returns(values...)
	m.doApply(m.imp)
	return when
}

// Origin 调用原函数
// origin 需要和原函数的参数列表保持一致
func (m *DefMocker) Origin(originFunc interface{}) ExportedMocker {
	m.origin = originFunc
	return m
}
