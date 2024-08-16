package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/builder"
	"github.com/cordialsys/crosschain/chain/template/tx_input"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {

	builder1, err := builder.NewTxBuilder(&xc.ChainConfig{})
	require.NotNil(t, builder1)
	require.EqualError(t, err, "not implemented")
}

func TestNewNativeTransfer(t *testing.T) {

	builder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder.NewNativeTransfer(from, to, amount, input)
	require.Nil(t, tf)
	require.EqualError(t, err, "not implemented")
}

func TestNewTokenTransfer(t *testing.T) {

	builder1, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder1.NewTokenTransfer(from, to, amount, input)
	require.Nil(t, tf)
	require.EqualError(t, err, "not implemented")
}

func TestNewTransfer(t *testing.T) {

	builder1, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tf, err := builder1.NewTransfer(from, to, amount, input)
	require.Nil(t, tf)
	require.EqualError(t, err, "not implemented")
}
