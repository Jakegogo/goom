# GOOM单测Mock框架
## 动态
- 开源版本将持续进行维护，欢迎提交issue
- 或者加入QQ群沟通

## 介绍
### 背景
1. 基于腾讯公司内部需要一款适合公司业务迭代速度和稳定性要求的mock库，国内众多项目采用外界开源的gomonkey框架进行函数的mock，因其版本更新不及时, 加上团队目前实现一款改进版-支持异包未导出函数mock，同包未导出方法mock，无需unpath即可在mock过程中调用原函数的特性，可以支持到延迟模拟，参数更改，mock数据录制等功能，因此建立此项目
2. 目前有一半以上方案是基于gomock类似的实现方案, 此mock方案需要要求业务代码具备良好的接口设计，才能顺利生成mock代码，而goom只需要指定函数名称或函数定义，就能支持到任意函数的mock，任意函数异常注入，延时模拟等扩展性功能
3. 当前实现已经支持Mac、Mac(ARM)、linux、windows等多种平台，支持go1.16-go1.23等各个版本, 解决permission_denied等问题，且对未来版本友好支持

### 功能特性
1. mock过程中调用原函数(线程安全, 支持并发单测)
2. 异常注入，对函数调用支持异常注入，延迟模拟等稳定性测试
3. 所有操作都是并发安全的
4. 未导出(未导出)函数(或方法)的mock(不建议使用, 对于未导出函数的Mock 通常都是因为代码设计可能有问题, 此功能会在未来版本中废弃)
5. 支持M1 mac环境运行，支持IDE debug，函数、方法mock，接口mock，未导出函数mock，等能力均可在arm64架构上使用

### 将来
1. 支持数据驱动测试
2. 支持Mock锚点定义
3. 支持代码重构

## 注意！！！不要过度依赖mock

> [1.千万不要过度依赖于mock](https://mp.weixin.qq.com/s?__biz=MzA5MTAzNjU1OQ==&mid=2454780683&idx=1&sn=aabc85f3bd2cfa21b8b806bad581f0c5)
>
> 2.对于正规的第三方库，比如mysql、gorm的库本身会提供mock能力, 可参考[sql_test.go](https://github.com/Tencent/goom/wiki/sql-mock%E6%A1%88%E4%BE%8B)
>
> 3.对于自建的内部依赖库, 建议由库的提供方编写mock(1.使用方无需关心提供方的实现细节、2.由库提供方负责版本升级时mock实现逻辑的更新)

## Install
```bash
# 支持的golang版本: go1.11-go1.18
go get github.com/tencent/goom
```

## Tips
```
注意: 按照go编译规则，短函数会被内联优化，导致无法mock的情况，编译参数需要加上 -gcflags=all=-l 关闭内联
例如: go test -gcflags=all=-l hello.go
```

## Getting Start
```golang
// 在需要使用mock的测试文件import
import "github.com/tencent/goom"
```
### 1. 基本使用
#### 1.1. 函数mock
```golang
// foo 函数定义如下
func foo(i int) int {
    //...
    return 0
}

// mock示例
// 创建当前包的mocker
mock := mocker.Create()

// mock函数foo并设定返回值为1
mock.Func(foo).Return(1)
s.Equal(1, foo(0), "return result check")

// 可搭配When使用: 参数匹配时返回指定值
mock.Func(foo).When(1).Return(2)
s.Equal(2, foo(1), "when result check")

// 使用arg.In表达式,当参数为1、2时返回值为100
mock.Func(foo).When(arg.In(1, 2)).Return(100)
s.Equal(100, foo(1), "when in result check")
s.Equal(100, foo(2), "when in result check")

// 按顺序依次返回(等价于gomonkey的Sequence)
mock.Func(foo).Returns(1, 2, 3)
s.Equal(1, foo(0), "returns result check")
s.Equal(2, foo(0), "returns result check")
s.Equal(3, foo(0), "returns result check")

// mock函数foo，使用Apply方法设置回调函数
// 注意: Apply和直接使用Return都可以实现mock，两种方式二选一即可
// Apply可以在桩函数内部实现自己的逻辑，比如根据不同参数返回不同值等等。
mock.Func(foo).Apply(func(int) int {
    return 1
})
s.Equal(1, foo(0), "apply callback check")


// bar 多参数函数
func bar(i interface{}, j int) int {
    //...
    return 0
}

// 忽略第一个参数, 当第二个参数为1、2时返回值为100
mock.Func(bar).When(arg.Any(), arg.In(1, 2)).Return(100)
s.Equal(100, bar(-1, 1), "any param result check")
s.Equal(100, bar(0, 1), "any param result check")
s.Equal(100, bar(1, 2), "any param result check")
s.Equal(100, bar(999, 2), "any param result check")
```

#### 1.2. 结构体方法mock
```golang
// 结构体定义如下
type Struct1 struct{
}

// Call 导出方法
func (f *Struct1) Call(i int) int {
    return i
}

// mock示例
// 创建当前包的mocker
mock := mocker.Create()

// mock 结构体Struct1的方法Call并设置其回调函数
// 注意: 当使用Apply方法时，如果被mock对象为结构体方法, 那么Apply参数func()的第一个参数必须为接收体(即结构体/指针类型)
// 其中, func (f *Struct1) Call(i int) int 和 &Struct1{} 与 _ *Struct1同时都是带指针的接受体类型, 需要保持一致
mock.Struct(&Struct1{}).Method("Call").Apply(func(_ *Struct1, i int) int {
    return i * 2
 })

// mock 结构体struct1的方法Call并返回1
// 简易写法直接Return方法的返回值, 无需关心方法签名
mock.Struct(&Struct1{}).Method("Call").Return(1)
```

#### 1.3. 结构体的未导出方法mock
```golang

// call 未导出方法示例
func (f *Struct1) call(i int) int {
    return i
}

// mock 结构体Struct1的未导出方法call, mock前先调用ExportMethod将其导出，并设置其回调函数
mock.Struct(&Struct1{}).ExportMethod("call").Apply(func(_ *Struct1, i int) int {
    return i * 2
})

// mock 结构体Struct1的未导出方法call, mock前先调用ExportMethod将其导出为函数类型，后续支持设置When, Return等
// As调用之后，请使用Return或When API的方式来指定mock返回。
mock.Struct(&Struct1{}).ExportMethod("call").As(func(_ *Struct1, i int) int {
    // 随机返回值即可; 因后面已经使用了Return,此函数不会真正被调用, 主要用于指定未导出函数的参数签名
    return i * 2
}).Return(1)
```

### 2. 接口Mock
接口定义举例:
```golang
// I 接口测试
type I interface {
  Call(int) int
  Call1(string) string
  call2(int32) int32
}
```

被测接口实例代码:
```golang
// TestTarget 被测对象
type TestTarget struct {
	// field 被测属性(接口类型)
	field I
}

func NewTestTarget(i I) *TestTarget {
	return &TestTarget{
		field:i,
	}
}

func (t *TestTarget) Call(num int) int {
	return field.Call(num)
}

func (t *TestTarget) Call1(str string) string {
    return  field.Call1(str)
}
```

接口属性/变量Mock示例:
```golang
mock := mocker.Create()

// 初始化接口变量
i := (I)(nil)

// 将Mock应用到接口变量
// 1. interface mock只对mock.Interface(&目标接口变量) 的目标接口变量生效, 因此需要将被测逻辑结构中的I类型属性或变量替换为i,mock才可生效
// 2. 一般建议使用struct mock即可。
// 3. Apply调用的第一个参数必须为*mocker.IContext, 作用是指定接口实现的接收体; 后续的参数原样照抄。
mock.Interface(&i).Method("Call").Apply(func(ctx *mocker.IContext, i int) int {
    return 100
})

// ===============================================================================
// !!! 如果是mock interface的话，需要将interface i变量赋值替换【被测对象】的【属性】,才能生效
// 也就是说,不对该接口的所有实现类实例生效。
t := NewTestTarget(i)

// 断言mock生效
s.Equal(100, t.Call(1), "interface mock check")

mock.Interface(&i).Method("Call1").As(func(ctx *mocker.IContext, i string) string {
    // 随机返回值即可; 因后面已经使用了Return,此函数不会真正被调用, 主要用于指定未导出函数的参数签名
	return ""
}).When("").Return("ok")
s.Equal("ok", t.Call1(""), "interface mock check")

// Mock重置, 接口变量将恢复原来的值
mock.Reset()
s.Equal(nil, i, "interface mock reset check")
```

### 3. 高阶用法
#### 3.1. 外部package的未导出函数mock(一般不建议对不同包下的未导出函数进行mock)
```golang
// 针对其它包的mock示例
// 创建指定包的mocker，设置引用路径
mock := mocker.Create()

// mock函数foo1并设置其回调函数
mock.Pkg("github.com/tencent/goom_test").ExportFunc("foo1").Apply(func(i int) int {
    return i * 3
})

// mock函数foo1并设置其返回值
mock.ExportFunc("foo1").As(func(i int) int {
    // 随机返回值即可; 因后面已经使用了Return,此函数不会真正被调用, 主要用于指定未导出函数的参数签名
    return 0
}).Return(1)
```

#### 3.2. 外部package的未导出结构体的mock(一般不建议对不同包下的未导出结构体进行mock)
```golang
// 针对其它包的mock示例
package https://github.com/tencent/goom/a

// struct2 要mock的目标结构体
type struct2 struct {
    field1 <type>
    // ...
}

```

Mock代码示例:
```golang
package https://github.com/tencent/goom/b

// fake fake一个结构体, 用于作为回调函数的Receiver
type fake struct {
    // fake结构体要和原未导出结构体的内存结构对齐
    // 即: 字段个数、顺序、类型必须一致; 比如: field1 <type> 如果有
    // 此结构体无需定义任何方法
	field1 <type>
    // ...
}

// 创建指定包的mocker，设置引用路径
mock := mocker.Create()

// mock其它包的未导出结构体struct2的未导出方法call，并设置其回调函数
// 如果参数是未导出的，那么需要在当前包fake一个同等结构的struct(只需要fake结构体，方法不需要fake)，fake结构体要和原未导出结构体struct2的内存结构对齐
// 注意: 如果方法是指针方法，那么需要给struct加上*，比如:ExportStruct("*struct2")
mock.Pkg("https://github.com/tencent/goom/a").ExportStruct("struct2").Method("call").Apply(func(_ *fake, i int) int {
    return 1
})
s.Equal(1, struct2Wrapper.call(0), "unexported struct mock check")

// mock其它包的未导出结构体struct2的未导出方法call，并设置其返回值
mock.ExportStruct("struct2").Method("call").As(func(_ *fake, i int) int {
	// 随机返回值即可; 因后面已经使用了Return,此函数不会真正被调用, 主要用于指定接口方法的参数签名
    return 0
}).Return(1) // 指定返回值
s.Equal(1, struct2Wrapper.call(0), "unexported struct mock check")
```

### 4. 追加多个返回值序列
```golang
mock := mocker.Create()

// 设置函数foo当传入参数为1时，第一次返回3，第二次返回2
when := mock.Func(foo).When(1).Return(0)
for i := 1;i <= 100;i++ {
    when.AndReturn(i)
}
s.Equal(0, foo(1), "andReturn result check")
s.Equal(1, foo(1), "andReturn result check")
s.Equal(2, foo(1), "andReturn result check")
 ...
```

### 5. 在回调函数中调用原函数
```golang
mock := mocker.Create()

// 定义原函数,用于占位,实际不会执行该函数体
// 需要和原函数的参数列表保持一致
// 定义原函数,用于占位,实际不会执行该函数体
var origin = func(i int) int {
    // 用于占位, 实际不会执行该函数体; 因底层trampoline技术的占位要求, 必须编写方法体
    fmt.Println("only for placeholder, will not call")
    // return 指定随机返回值即可
    return 0
}

mock.Func(foo1).Origin(&origin).Apply(func(i int) int {
    // 调用原函数
    originResult := origin(i)

    // 加入延时逻辑等
    time.Sleep(time.Seconds)

    return originResult + 100
})
// foo1(1) 等待1秒之后返回:101
s.Equal(101, foo1(1), "call origin result check")
```

## 问题答疑
常见问题:
1. 如果是M1-MAC(arm CPU)机型, 可以尝试以下两种方案

a. 尝试使用权限修复工具,在项目根目录执行以下指令:
```shell
MOCKER_DIR=$(go list -m -f '{{.Dir}}' github.com/tencent/goom)
${MOCKER_DIR}/tool/permission_denied.sh -i
```

b: 如果a方案没有效果，则尝试切换成amd的go编译器,在环境变量中添加:
```shell
GOARCH=amd64
```

2. 如果遇到mock未生效的问题,可以打开debug日志进行自助排查
```go
// TestUnitTestSuite 测试入口
func TestUnitTestSuite(t *testing.T) {
	// 开启debug模式, 在控制台可以
	// 1.查看apply和reset的状态日志
	// 2.查看mock调用日志
	mocker.OpenDebug()
	suite.Run(t, new(mockerTestSuite))
}
```
3. windows系统下,请加上构建参数以打开符号表编译: -ldflags="-s=false", 比如
```shell
go test -ldflags="-s=false" -gcflags "all=-N -l" ./...
```


4. go 1.23以上版本需要加上以下构建参数才可以使用    
报错内容:
```
link: github.com/tencent/goom/internal/hack: invalid reference to runtime.firstmoduledata
```
解决方案1，添加构建参数:
```shell
-gcflags="all=-N -l" -ldflags=-checklinkname=0
```
> -gcflags="all=-N -l": 解决被mock函数内联问题，和permission denied的问题(go1.23版本新增要求)  
> -checklinkname=0: 关闭golinkname的标签检查，即继续允许go:linkname标签的使用   


解决方案2: 升级到最新版本:
v1.0.4-rc1
此版本处于公测阶段，目前自测在windows、mac、mac(ARM)、linux，go1.16-go1.23版本均可使用
未来更高的go版本，理论上也能较好支持


## 联系答疑
常见问题可参考: https://github.com/Tencent/goom/wiki

或者提issue: https://github.com/Tencent/goom/issues


## Contributor
@yongfuchen、@adrewchen、@bingjgyan、@mingjiehu、@ivyyi、@miliao


# QQ群
<img src="https://github.com/user-attachments/assets/ae5fc289-f47c-45c8-b8c2-688cffe8eda0" alt="扫码加QQ群" width="350" />
