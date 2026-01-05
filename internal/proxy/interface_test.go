// Package proxy_test 对 proxy 包的测试
package proxy_test

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/tencent/goom/internal/hack"
	"github.com/tencent/goom/internal/iface"
	"github.com/tencent/goom/internal/logger"
	"github.com/tencent/goom/internal/proxy"
	"github.com/tencent/goom/internal/unexports2"
)

// I 接口测试
type I interface {
	Call(int) int
	Call1(string) string
	call2(int32) int32
}

// TestInterfaceCall 测试接口调用
func TestInterfaceCall(t *testing.T) {
	i := getImpl(1)
	i.Call(99)
}

func foo(a int) int {
	return a + 1
}

// TestMakeFunc 测试 MakeFunc
func TestMakeFunc(t *testing.T) {
	funcValue := reflect.ValueOf(foo)
	funcType := funcValue.Type()
	mockFunc := reflect.MakeFunc(funcType, func(args []reflect.Value) (results []reflect.Value) {
		fmt.Println("called, args: ", args[0].Interface())
		return funcValue.Call(args)
	})
	fun := mockFunc.Interface()
	func1, _ := (fun).(func(int) int)
	if func1(3) != 4 {
		t.Errorf("func1 return expect %d", 4)
	}
}

// TestAutoGen 测试生成任意接口实现
func TestAutoGen(t *testing.T) {
	logger.LogLevel = logger.DebugLevel
	logger.SetLog2Console(true)
	const strResult = "not ok"

	gen := (I)(nil)

	ctx := iface.NewContext()

	_ = proxy.Interface(&gen, ctx, "Call", func(ctx *iface.IContext, a int) int {
		t.Log("called Call")
		return 1
	}, nil)

	_ = proxy.Interface(&gen, ctx, "Call1", func(ctx *iface.IContext, a string) string {
		t.Log("called Call1")
		return strResult
	}, nil)

	_ = proxy.Interface(&gen, ctx, "call2", func(ctx *iface.IContext, a int32) int32 {
		t.Log("called call2")
		return 99
	}, nil)

	// 调用接口方法
	if r := gen.Call(2); r != 1 {
		t.Fatalf("want result: %d, real: %d", 1, r)
	}
	if r := gen.Call1("ok"); r != strResult {
		t.Fatalf("want result: %s, real: %s", "not ok", r)
	}
	if r := gen.call2(33); r != 99 {
		t.Fatalf("want result: %d, real: %d", 99, r)
	}
}

// TestGenCancel 测试取消接口代理
func TestGenCancel(t *testing.T) {
	gen := getImpl(1)
	ctx := iface.NewContext()

	_ = proxy.Interface(&gen, ctx, "Call", func(ctx *iface.IContext, a int) int {
		t.Log("called Call")
		return 0
	}, nil)

	if r := gen.Call(2); r != 0 {
		t.Fatalf("want result: %d, real: %d", 0, r)
	}

	ctx.Cancel()

	if r := gen.Call(0); r != 1 {
		t.Fatalf("want result: %d, real: %d", 1, r)
	}
}

// TestNilImpl 测试空实现结构体方法列表
func TestNilImpl(t *testing.T) {
	gen := (*I)(nil)
	typ := reflect.TypeOf(gen).Elem()

	for i := 0; i < typ.NumMethod(); i++ {
		fmt.Println(typ.Method(i).Name, typ.Method(i).Type)
	}
}

// TestGenImpl 测试生成接口实现
func TestGenImpl(t *testing.T) {
	gen := (I)(nil)
	typ := reflect.TypeOf(&gen).Elem()

	for i := 0; i < typ.NumMethod(); i++ {
		fmt.Println(typ.Method(i).Name, typ.Method(i).Type)
	}

	genInterfaceImpl(&gen, func(data *Impl2, a int) int {
		fmt.Println("proxy")
		return 3
	})

	// 调用接口方法
	r := (gen).Call(1)
	if r != 3 {
		t.Fatalf("want result: %d, real: %d", 3, r)
	}

	fmt.Println("ok", r)
}

// genInterfaceImpl 生成接口实现
func genInterfaceImpl(i interface{}, proxy interface{}) {
	gen := hack.UnpackEFace(i).Data
	// mock 接口方法
	mockFunc := reflect.ValueOf(proxy)
	ifc := *(*uintptr)(gen)
	fmt.Println(ifc)

	// 伪装 iface
	*(*hack.Iface)(gen) = hack.Iface{
		Tab: &hack.Itab{
			Fun: [hack.MaxMethod]uintptr{mockFunc.Pointer(), uintptr(0), uintptr(0)},
		},
		Data: unsafe.Pointer(&Impl2{
			field1: "ok",
		}),
	}
	ifc = *(*uintptr)(gen)
	fmt.Println(ifc)
}

// TestAutoProxyGenImpl 测试生成任意接口实现
func TestAutoProxyGenImpl(t *testing.T) {
	logger.LogLevel = logger.DebugLevel
	logger.SetLog2Console(true)

	gen := (I)(nil)

	dynamicGenImpl(t, &gen)

	// 调用接口方法
	if r := gen.Call(1); r != 3 {
		t.Fatalf("want result: %d, real: %d", 3, r)
	}

	fmt.Println("ok")
}

// dynamicGenImpl 生成任意接口实现
func dynamicGenImpl(t *testing.T, i interface{}) {
	typ := reflect.TypeOf(i).Elem()
	for i := 0; i < typ.NumMethod(); i++ {
		fmt.Println(typ.Method(i).Name, typ.Method(i).Type)
	}

	gen := hack.UnpackEFace(i).Data

	// mock 接口方法
	methodTyp := reflect.TypeOf(func(data *Impl2, a int) int {
		fmt.Println("proxy")
		return 3
	})

	mockFunc := reflect.MakeFunc(methodTyp, func(args []reflect.Value) (results []reflect.Value) {
		return []reflect.Value{reflect.ValueOf(3)}
	})
	ifc := *(*uintptr)(gen)
	fmt.Println(ifc)

	callStub, err := unexports2.FindFuncByName("reflect.makeFuncStub")
	mockFuncPtr := (*hack.Value)(unsafe.Pointer(&mockFunc)).Ptr
	genStub, err := iface.MakeMethodCallerWithCtx(mockFuncPtr, callStub)

	if err != nil {
		panic(err)
	}

	fmt.Printf("genstub: 0x%x callstub: 0x%x\n", genStub, callStub)

	// 伪装 iface
	*(*hack.Iface)(gen) = hack.Iface{
		Tab: &hack.Itab{
			Fun: [hack.MaxMethod]uintptr{genStub, uintptr(0), uintptr(0)},
		},
		Data: (*hack.Value)(unsafe.Pointer(&mockFunc)).Ptr,
	}
	ifc = *(*uintptr)(gen)

	fmt.Println(ifc)
	fmt.Println(uintptr(getPtr(reflect.ValueOf(mockFunc.Interface()))))
	fmt.Println(mockFunc.Pointer())
}

// getPtr 获取函数的调用地址(和函数的指令地址不一样)
func getPtr(v reflect.Value) unsafe.Pointer {
	return (*hack.Value)(unsafe.Pointer(&v)).Ptr
}

func getImpl(n int) I {
	if n == 1 {
		return &Impl1{}
	} else if n == 2 {
		return &Impl2{}
	}
	return nil
}

type Impl1 struct {
	// nolint
	field1 string
}

func (i Impl1) Call(a int) int {
	fmt.Println("Impl1 called ")
	return 1 + a
}

func (i Impl1) Call1(string) string {
	return "ok"
}

func (i Impl1) call2(int32) int32 {
	return 11
}

type Impl2 struct {
	field1 string
}

func (i Impl2) Call(a int) int {
	fmt.Println("Impl2 called ")
	return 2 + a
}

func (i Impl2) Call1(string) string {
	return "!ok"
}

func (i Impl2) call2(int32) int32 {
	return 22
}

// TestTraceBack 测试生成任意接口实现的 traceback
func TestTraceBack(t *testing.T) {
	gen := (I)(nil)
	dynamicGenImpl(t, &gen)

	// 调用接口方法
	for i := 0; i < 1000; i++ {
		gen.Call(1)
	}
	fmt.Println("ok")
}
