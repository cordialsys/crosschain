package sui_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestSuiTestSuite(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}
