package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/builder"
	"github.com/cordialsys/crosschain/chain/xlm/tx"
	"github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/stellar/go/xdr"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput
type Tx = tx.Tx

func TestNewTxBuilder(t *testing.T) {
	txBuilder, err := builder.NewTxBuilder(&xc.ChainConfig{Chain: "XLM"})
	require.NotNil(t, txBuilder)
	require.Nil(t, err)
}

func TestNewNativeTransfer(t *testing.T) {
	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{Chain: "XLM"})
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{}
	nt, err := txBuilder.NewNativeTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	var source xdr.MuxedAccount
	err = source.SetAddress(string(from))
	require.NoError(t, err)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	var destination xdr.MuxedAccount
	err = destination.SetAddress(string(to))
	require.NoError(t, err)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())
}
