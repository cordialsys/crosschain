package evm_legacy

import (
	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := NewTxBuilder(&xc.ChainConfig{})
	require.NoError(err)
	require.NotNil(builder)
}
