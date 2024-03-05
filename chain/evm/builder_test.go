package evm

import (
	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	builder, err := NewTxBuilder(&xc.TokenAssetConfig{Asset: "USDC", Contract: "1234"})
	require.Nil(err)
	require.NotNil(builder)
	require.Equal("USDC", builder.(TxBuilder).Asset.(*xc.TokenAssetConfig).Asset)
}

func (s *CrosschainTestSuite) TestTransferSetsMaxTipCap() {
	require := s.Require()
	builder, _ := NewTxBuilder(&xc.ChainConfig{})

	from := "0x724435CC1B2821362c2CD425F2744Bd7347bf299"
	to := "0x3ad57b83B2E3dC5648F32e98e386935A9B10bb9F"
	amount := xc.NewAmountBlockchainFromUint64(100)
	input := NewTxInput()

	input.GasTipCap = GweiToWei(DefaultMaxTipCapGwei - 1)
	tx, err := builder.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(err)
	require.EqualValues(GweiToWei(DefaultMaxTipCapGwei-1).Uint64(), tx.(*Tx).EthTx.GasTipCap().Uint64())

	input.GasTipCap = GweiToWei(DefaultMaxTipCapGwei + 1)
	tx, err = builder.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(err)
	require.EqualValues(GweiToWei(DefaultMaxTipCapGwei).Uint64(), tx.(*Tx).EthTx.GasTipCap().Uint64())

	// increase the max
	builder, _ = NewTxBuilder(&xc.ChainConfig{ChainMaxGasPrice: 100})
	tx, _ = builder.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	// now DefaultMaxTipCapGwei + 1 is used
	require.EqualValues(GweiToWei(DefaultMaxTipCapGwei+1).Uint64(), tx.(*Tx).EthTx.GasTipCap().Uint64())

	// 100 is used instead of 1000
	input.GasTipCap = GweiToWei(1000)
	tx, _ = builder.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.EqualValues(GweiToWei(100).Uint64(), tx.(*Tx).EthTx.GasTipCap().Uint64())
}
