// Package mocker_test 对 mocker 包的测试
// 当前文件实现了对 arm64 架构下 trampoline 调用原函数的单测
package mocker_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	mocker "github.com/tencent/goom"
	"github.com/tencent/goom/test"
)

// TestUnitArm64TestSuite 测试入口
func TestUnitArm64TestSuite(t *testing.T) {
	// 开启 debug
	// 1.可以查看 apply 和 reset 的状态日志
	// 2.查看 mock 调用日志
	mocker.OpenDebug()
	suite.Run(t, new(mockerTestArm64Suite))
}

type mockerTestArm64Suite struct {
	suite.Suite
	fakeErr error
}

func (s *mockerTestArm64Suite) SetupTest() {
	s.fakeErr = errors.New("fake error")
}

// TestCallOrigin 测试调用原函数 mock return
func (s *mockerTestArm64Suite) TestCallOrigin() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func(i int) int {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return 0
		}

		mock := mocker.Create()
		mock.Func(test.Foo).Origin(&origin).Apply(func(i int) int {
			s.NotNil(origin)
			originResult := origin(i)
			fmt.Printf("arguments are %v\n", i)
			return originResult + 100
		})
		s.Equal(101, test.Foo(1), "foo mock check")
		s.Equal(100, test.Foo(0), "foo mock check (boundary: 0)")
		s.Equal(-23+100, test.Foo(-23), "foo mock check (boundary: negative)")

		mock.Reset()
		s.Equal(1, test.Foo(1), "foo mock reset check")
	})
}

func (s *mockerTestArm64Suite) TestUnitEmptyMatch() {
	s.Run("empty return", func() {
		mocker.Create().Func(time.Sleep).Return()
		time.Sleep(time.Second)
	})
}

// TestCallOriginMultiReturn 测试多返回值函数的 Origin trampoline
func (s *mockerTestArm64Suite) TestCallOriginMultiReturn() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func() ([]byte, error) {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return nil, nil
		}
		mock := mocker.Create()
		defer mock.Reset()

		mock.Func(test.GetS).Origin(&origin).Apply(func() ([]byte, error) {
			s.NotNil(origin)
			b, err := origin()
			if err != nil {
				return nil, err
			}
			return append(b, []byte("-patched")...), nil
		})

		b, err := test.GetS()
		s.NoError(err)
		s.Equal("hello-patched", string(b))
	})
}

// TestCallOriginReturnPtr 测试返回指针的函数的 Origin trampoline
func (s *mockerTestArm64Suite) TestCallOriginReturnPtr() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func() *test.S {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return nil
		}
		mock := mocker.Create()
		defer mock.Reset()

		mock.Func(test.Foo1).Origin(&origin).Apply(func() *test.S {
			s.NotNil(origin)
			v := origin()
			s.NotNil(v)
			v.Field2 += 100
			return v
		})

		v := test.Foo1()
		s.Equal("ok", v.Field1)
		s.Equal(102, v.Field2)
	})
}

// TestCallOriginMethod 测试方法 mock + Origin trampoline（覆盖 InstanceMethodTrampoline 路径）
func (s *mockerTestArm64Suite) TestCallOriginMethod() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func(_ *test.Fake, i int) int {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return i
		}
		mock := mocker.Create()
		defer mock.Reset()

		mock.Struct(&test.Fake{}).Method("Call").Origin(&origin).Apply(func(f *test.Fake, i int) int {
			s.NotNil(origin)
			return origin(f, i) + 100
		})

		f := &test.Fake{}
		s.Equal(101, f.Call(1))
		s.Equal(-10001+100, f.Call(-10001), "boundary: large negative triggers dummy() branch")
	})
}

// TestCallOriginApplyResetReapply 测试 Apply/Reset/再次 Apply 的边界场景
func (s *mockerTestArm64Suite) TestCallOriginApplyResetReapply() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func(i int) int {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return 0
		}
		mock := mocker.Create()

		mock.Func(test.Foo).Origin(&origin).Apply(func(i int) int {
			return origin(i) + 10
		})
		s.Equal(11, test.Foo(1))

		mock.Reset()
		s.Equal(1, test.Foo(1))

		// re-apply with a new origin slot to ensure the trampoline wiring works repeatedly
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin2 = func(i int) int {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return 0
		}
		mock.Func(test.Foo).Origin(&origin2).Apply(func(i int) int {
			return origin2(i) + 20
		})
		s.Equal(21, test.Foo(1))

		mock.Reset()
		s.Equal(1, test.Foo(1))
	})
}

// TestCallOriginConcurrent 并发调用 smoke test（不做 data race 断言，但确保不 panic 且结果可预测）
func (s *mockerTestArm64Suite) TestCallOriginConcurrent() {
	s.Run("success", func() {
		// 定义原函数,用于占位,实际不会执行该函数体
		var origin = func(i int) int {
			// 用于占位,实际不会执行该函数体, 但是必须编写
			fmt.Println("only for placeholder, will not call")
			// return 任意值
			return 0
		}
		mock := mocker.Create()
		defer mock.Reset()

		mock.Func(test.Foo).Origin(&origin).Apply(func(i int) int {
			return origin(i) + 1
		})

		const (
			g = 20
			n = 200
		)
		var sum int64
		var wg sync.WaitGroup
		wg.Add(g)
		for gi := 0; gi < g; gi++ {
			go func(base int) {
				defer wg.Done()
				for j := 0; j < n; j++ {
					v := test.Foo(base + j)
					atomic.AddInt64(&sum, int64(v))
				}
			}(gi * 1000)
		}
		wg.Wait()
		s.NotZero(sum)
	})
}
