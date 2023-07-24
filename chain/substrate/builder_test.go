package substrate

import (
	xc "github.com/jumpcrypto/crosschain"
)

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := NewTxBuilder(&xc.AssetConfig{})
	require.Nil(err)
	require.NotNil(builder)
}

func (s *CrosschainTestSuite) TestNewTransferFail() {
	require := s.Require()
	builder, _ := NewTxBuilder(&xc.AssetConfig{})
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
