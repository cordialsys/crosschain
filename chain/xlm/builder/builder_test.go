package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/xlm/builder"
	"github.com/cordialsys/crosschain/chain/xlm/common"
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
	args := buildertest.MustNewTransferArgs(from, to, amount)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	destination := common.MustMuxedAccountFromAddres(to)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())
}

func TestNewTokenTransfer(t *testing.T) {
	txBuilder, _ := builder.NewTxBuilder(
		&xc.TokenAssetConfig{
			Contract: "USDC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5",
			ChainConfig: &xc.ChainConfig{
				Chain: "XLM",
			},
		},
	)
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{}
	args := buildertest.MustNewTransferArgs(from, to, amount)
	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)

	txEnvelope := nt.(*Tx).TxEnvelope
	source := common.MustMuxedAccountFromAddres(from)
	require.Equal(t, txEnvelope.SourceAccount(), source)

	destination := common.MustMuxedAccountFromAddres(to)
	payment, ok := txEnvelope.Operations()[0].Body.GetPaymentOp()
	require.NotZero(t, ok)
	require.Equal(t, payment.Destination, destination)
	require.Equal(t, payment.Asset.AlphaNum4.AssetCode, xdr.AssetCode4{'U', 'S', 'D', 'C'})
	require.Equal(t, payment.Asset.AlphaNum4.Issuer.Address(), "GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5")

	require.Equal(t, int64(payment.Amount), amount.Int().Int64())
}

func TestInvalidTokenTransfer(t *testing.T) {
	txBuilder, _ := builder.NewTxBuilder(
		&xc.TokenAssetConfig{
			// Asset code is too long
			Contract: "USDCCCCCCCCCCCCCCCCCCCCCCCCC-GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5",
			ChainConfig: &xc.ChainConfig{
				Chain: "XLM",
			},
		},
	)
	from := xc.Address("GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H")
	to := xc.Address("GCITKPHEIYPB743IM4DYB23IOZIRBAQ76J6QNKPPXVI2N575JZ3Z65DI")
	amount := xc.NewAmountBlockchainFromUint64(10)
	input := &tx_input.TxInput{}

	args := buildertest.MustNewTransferArgs(from, to, amount)
	_, err := txBuilder.Transfer(args, input)
	require.Error(t, err)
}
