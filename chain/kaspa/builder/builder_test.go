package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/template/builder"
	"github.com/cordialsys/crosschain/chain/template/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(xc.NewChainConfig("XYZ").Base())
	require.NotNil(t, builder1)
	require.EqualError(t, err, "not implemented")
}

func TestNewNativeTransfer(t *testing.T) {
	builder, _ := builder.NewTxBuilder(xc.NewChainConfig("XYZ").Base())
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	chainCfg := xc.NewChainConfig(xc.KAS).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
	)
	_, err := builder.Transfer(args, input)
	require.ErrorContains(t, err, "not implemented")
}

func TestNewTokenTransfer(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("XYZ").Base())
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	chainCfg := xc.NewChainConfig(xc.KAS).Base()
	args := buildertest.MustNewTransferArgs(
		chainCfg, from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress("contract")),
	)
	_, err := builder1.Transfer(args, input)
	require.ErrorContains(t, err, "token transfers are not supported for XYZ")
}
