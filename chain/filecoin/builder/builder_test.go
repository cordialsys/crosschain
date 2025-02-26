package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/filecoin/builder"
	"github.com/cordialsys/crosschain/chain/filecoin/tx_input"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(&xc.ChainConfig{Chain: "FIL"})
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestNewTransfer(t *testing.T) {
	builder1, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("f1urvqy4hx5idlki6b6f7ab6hzihjdfy47b5cc6dy")
	to := xc.Address("f13uhmulxtag3qfohj7h2nmtco7e7u3t3nxjdzi7q")
	amount := xc.NewAmountBlockchainFromStr("1")
	input := &TxInput{
		Nonce:      0,
		GasLimit:   100000,
		GasFeeCap:  xc.NewAmountBlockchainFromStr("150000"),
		GasPremium: xc.NewAmountBlockchainFromStr("250000"),
	}

	tf, err := builder1.NewTransfer(from, to, amount, input)
	require.NotNil(t, tf)
	require.NoError(t, err)
}
