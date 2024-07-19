package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/stretchr/testify/require"
)

func TestNewTxBuilder(t *testing.T) {
	b, err := builder.NewTxBuilder(&xc.TokenAssetConfig{Asset: "USDC", Contract: "1234"})
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, "USDC", b.(builder.TxBuilder).Asset.(*xc.TokenAssetConfig).Asset)
}

func TestTransferSetsMaxTipCap(t *testing.T) {
	b, _ := builder.NewTxBuilder(&xc.ChainConfig{})

	from := "0x724435CC1B2821362c2CD425F2744Bd7347bf299"
	to := "0x3ad57b83B2E3dC5648F32e98e386935A9B10bb9F"
	amount := xc.NewAmountBlockchainFromUint64(100)
	input := tx_input.NewTxInput()

	input.GasTipCap = builder.GweiToWei(builder.DefaultMaxTipCapGwei - 1)
	trans, err := b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(t, err)
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei-1).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	input.GasTipCap = builder.GweiToWei(builder.DefaultMaxTipCapGwei + 1)
	trans, err = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(t, err)
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	// increase the max
	b, _ = builder.NewTxBuilder(&xc.ChainConfig{ChainMaxGasPrice: 100})
	trans, _ = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	// now DefaultMaxTipCapGwei + 1 is used
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei+1).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	// 100 is used instead of 1000
	input.GasTipCap = builder.GweiToWei(1000)
	trans, _ = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.EqualValues(t, builder.GweiToWei(100).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())
}
