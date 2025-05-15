package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/filecoin/builder"
	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(xc.NewChainConfig(xc.FIL).Base())
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestNewTransfer(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy")
	to := xc.Address("f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q")
	amount := xc.NewAmountBlockchainFromStr("1")
	input := &TxInput{
		Nonce:      0,
		GasLimit:   100000,
		GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
		GasPremium: xc.NewAmountBlockchainFromStr("250000"),
	}

	tf, err := builder1.Transfer(buildertest.MustNewTransferArgs(from, to, amount), input)
	require.NoError(t, err)
	require.NotNil(t, tf)
}
