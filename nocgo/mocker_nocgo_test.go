package nocgo_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	mocker "github.com/tencent/goom"
)

// foo foo 测试函数
//
//go:noinline
func foo(i int) int {
	// check 对 defer 的支持
	defer func() { fmt.Printf("defer\n") }()
	// cgoFuncAny()
	return i * 1
}

// TestUnitBuilderNoCGOTestSuite 测试入口, 测试没有引入cgo的情况
func TestUnitBuilderNoCGOTestSuite(t *testing.T) {
	mocker.OpenDebug()
	suite.Run(t, new(mockerNoCGOTestSuite))
}

type mockerNoCGOTestSuite struct {
	suite.Suite
	fakeErr error
}

func (s *mockerNoCGOTestSuite) SetupTest() {
	s.fakeErr = errors.New("fake error")
}

// TestMultiReturn 测试调用原函数多返回
func (s *mockerNoCGOTestSuite) TestMultiReturn() {
	s.Run("success", func() {
		mock := mocker.Create()
		mock.Func(foo).When(1).Return(3).AndReturn(2)
		s.Equal(3, foo(1), "foo mock check")
		s.Equal(2, foo(1), "foo mock check")

		mock.Reset()
		s.Equal(1, foo(1), "foo mock reset check")
	})
}
