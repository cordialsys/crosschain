package aptos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AptosTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *AptosTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestAptosTestSuite(t *testing.T) {
	suite.Run(t, new(AptosTestSuite))
}
