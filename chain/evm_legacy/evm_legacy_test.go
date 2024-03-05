package evm_legacy

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

func TestLegacyEvm(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}
