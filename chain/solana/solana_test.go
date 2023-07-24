package solana

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SolanaTestSuite struct {
	suite.Suite
	Ctx context.Context
}

func (s *SolanaTestSuite) SetupTest() {
	s.Ctx = context.Background()
}

func TestSolana(t *testing.T) {
	suite.Run(t, new(SolanaTestSuite))
}
