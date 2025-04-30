package builder_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/xrp/builder"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput
type Tx = tx.Tx

func TestNewTxBuilder(t *testing.T) {

	txBuilder, err := builder.NewTxBuilder(xc.NewChainConfig("XRP").Base())
	require.NotNil(t, txBuilder)
	require.Nil(t, err)
}

func TestNewNativeTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &tx_input.TxInput{}

	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionMemo("999"),
		buildertest.OptionPublicKey(make([]byte, 32)),
	)

	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)
	xrpTx := nt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.XRPAmount, "12")
	require.EqualValues(t, xrpTx.DestinationTag, 999)
}

func TestNewNativeTransferAccountDelete(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &tx_input.TxInput{
		AccountDelete:    true,
		AccountDeleteFee: xc.NewAmountBlockchainFromUint64(200_000),
		Fee:              xc.NewAmountBlockchainFromUint64(100),
	}

	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionMemo("999"),
		buildertest.OptionPublicKey(make([]byte, 32)),
		buildertest.OptionInclusiveFeeSpending(true),
	)

	nt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, nt)
	xrpTx := nt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, "200000", string(xrpTx.Fee))
	require.EqualValues(t, "AccountDelete", xrpTx.TransactionType)
	require.EqualValues(t, 999, xrpTx.DestinationTag)

	// Ensure it's not possible to use account-delete without inclusive fee spending
	args.SetInclusiveFeeSpending(false)
	nt, err = txBuilder.Transfer(args, input)
	require.NoError(t, err)
	xrpTx = nt.(*Tx).XRPTx
	require.EqualValues(t, "Payment", xrpTx.TransactionType)
}

func TestNewTokenTransfer(t *testing.T) {

	contract := "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB"
	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12000000000000000)
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 15),
		buildertest.OptionMemo("0x999"),
		buildertest.OptionPublicKey(make([]byte, 32)),
	)

	input := &TxInput{}
	tt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tt)
	xrpTx := tt.(*Tx).XRPTx
	require.Equal(t, xrpTx.Amount.TokenAmount.Value, "12")
	require.Equal(t, xrpTx.Amount.TokenAmount.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.TokenAmount.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
	require.EqualValues(t, xrpTx.DestinationTag, 0x999)
}

func TestNewTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &TxInput{}
	args := buildertest.MustNewTransferArgs(
		from, to, amount,
		buildertest.OptionPublicKey(make([]byte, 32)),
	)
	tnt, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tnt)
	xrpTx := tnt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.XRPAmount, "12")
	require.EqualValues(t, xrpTx.DestinationTag, 0)

	contract := "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB"
	amount2 := xc.NewAmountBlockchainFromUint64(12000000000000000)

	args = buildertest.MustNewTransferArgs(
		from, to, amount2,
		buildertest.OptionContractAddress(xc.ContractAddress(contract), 15),
		buildertest.OptionPublicKey(make([]byte, 32)),
	)
	tnt, err = txBuilder.Transfer(args, input)
	require.NoError(t, err)
	require.NotNil(t, tnt)
	xrpTx = tnt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.TokenAmount.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.TokenAmount.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
	require.Equal(t, xrpTx.Amount.TokenAmount.Value, "12")
}
