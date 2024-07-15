package substrate_test

import (
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/substrate"
)

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := substrate.NewTxBuilder(&xc.ChainConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestNewTransferFail() {
	require := s.Require()
	builder, _ := substrate.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(10000000000)
	input := &TxInput{} // missing metadata
	_, err := builder.NewTransfer(from, to, amount, input)
	require.NotNil(err)
}

func (s *CrosschainTestSuite) TestNewTransfer() {
	// TODO: write test
}
