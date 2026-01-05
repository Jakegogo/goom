package mocker_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/tencent/goom"
	"github.com/tencent/goom/arg"
	"github.com/tencent/goom/test"
)

func TestNewClients(t *testing.T) {
	suite.Run(t, new(NewClientsTestSuite))
}

type NewClientsTestSuite struct {
	suite.Suite
}

func (s *NewClientsTestSuite) SetupTest() {
	mocker.OpenDebug()
}

func (s *NewClientsTestSuite) TestNewClients() {
	s.Run("success", func() {
		mock := mocker.Create()
		mock.Func(test.NewClient).Return(nil, errors.New("failure"))
		c, e := test.NewClient()
		s.Equal((*test.Client)(nil), c)
		s.Equal(errors.New("failure"), e)
	})
}

func (s *NewClientsTestSuite) TestNewClientsWhen() {
	s.Run("success", func() {
		op := test.SetHttpClient()

		mock := mocker.Create()
		//mock.Func(elastic.NewClient).Return(((*elastic.Client)(nil), errors.New("failure")))
		mock.Func(test.NewClient).Return(nil, errors.New("default")).
			When(op, op).Return((*test.Client)(nil), errors.New("failure"))

		c, e := test.NewClient(op, op)
		s.Equal((*test.Client)(nil), c)
		s.Equal(errors.New("failure"), e)
	})
}

func (s *NewClientsTestSuite) TestNewClientsIn() {
	s.Run("success", func() {
		op := test.SetHttpClient()

		mock := mocker.Create()
		mock.Func(test.NewClient).Return(nil, errors.New("default")).
			In([]test.ClientOptionFunc{op, op}, []test.ClientOptionFunc{op, op}).Return(nil, errors.New("failure"))

		c, e := test.NewClient(op, op)
		s.Equal((*test.Client)(nil), c)
		s.Equal(errors.New("failure"), e)
	})
}

func (s *NewClientsTestSuite) TestNewClientsWhenIn() {
	s.Run("success", func() {
		op := test.SetHttpClient()

		mock := mocker.Create()
		mock.Func(test.NewClient).Return(nil, errors.New("default")).
			When(arg.In(op), arg.In(op)).Return(nil, errors.New("failure"))

		c, e := test.NewClient(op, op)
		s.Equal((*test.Client)(nil), c)
		s.Equal(errors.New("failure"), e)
	})
}
