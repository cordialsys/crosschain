package builder_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/builder"
	"github.com/cordialsys/crosschain/chain/xrp/tx"
	"github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput
type Tx = tx.Tx

func TestNewTxBuilder(t *testing.T) {

	txBuilder, err := builder.NewTxBuilder(&xc.ChainConfig{Chain: "XRP"})
	require.NotNil(t, txBuilder)
	require.Nil(t, err)
}

func TestNewNativeTransfer(t *testing.T) {

	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &tx_input.TxInput{}
	nt, err := txBuilder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, nt)
	xrpTx := nt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.StringValue, "12")
}

func TestNewNativeTransferErr(t *testing.T) {

	// invalid from, to
	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &tx_input.TxInput{}
	nt, err := txBuilder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.Nil(t, nt)
	require.EqualError(t, err, "failed to serialize transaction for signing `from` is an invalid classic address")

	from = xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	nt, err = txBuilder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.Nil(t, nt)
	require.EqualError(t, err, "failed to serialize transaction for signing `to` is an invalid classic address")
}

func TestNewTokenTransfer(t *testing.T) {

	contract := "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB"
	txBuilder, _ := builder.NewTxBuilder(&xc.TokenAssetConfig{
		Contract:    contract,
		Decimals:    6,
		ChainConfig: &xc.ChainConfig{},
	})
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &TxInput{}
	tt, err := txBuilder.(xc.TxTokenBuilder).NewTokenTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tt)
	xrpTx := tt.(*Tx).XRPTx
	require.Equal(t, xrpTx.Amount.AmountValue.Value, "12")
	require.Equal(t, xrpTx.Amount.AmountValue.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.AmountValue.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
}

func TestNewTokenTransferErr(t *testing.T) {

	// invalid asset
	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("from")
	to := xc.Address("to")
	amount := xc.AmountBlockchain{}
	input := &TxInput{}
	tt, err := txBuilder.(xc.TxTokenBuilder).NewTokenTransfer(from, to, amount, input)
	require.Nil(t, tt)
	require.EqualError(t, err, "asset does not have a contract")

	// invalid from, to
	txBuilder, _ = builder.NewTxBuilder(&xc.TokenAssetConfig{
		Contract: "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB",
		Decimals: 6,
	})
	from = xc.Address("from")
	to = xc.Address("to")
	amount = xc.AmountBlockchain{}
	input = &TxInput{}
	tt, err = txBuilder.(xc.TxTokenBuilder).NewTokenTransfer(from, to, amount, input)
	require.Nil(t, tt)
	require.EqualError(t, err, "failed to serialize transaction for signing `from` is an invalid classic address")

	from = xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	tt, err = txBuilder.(xc.TxTokenBuilder).NewNativeTransfer(from, to, amount, input)
	require.Nil(t, tt)
	require.EqualError(t, err, "failed to serialize transaction for signing `to` is an invalid classic address")
}

func TestNewTransfer(t *testing.T) {

	builder1, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &TxInput{}
	tnt, err := builder1.NewTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tnt)
	xrpTx := tnt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.StringValue, "12")
	encodeForSigningNative := tnt.(*Tx).EncodeForSigning
	require.Equal(t, hex.EncodeToString(encodeForSigningNative), "5354580012000022000000002400000000201b0000000061400000000000000c68400000000000000a73008114f667b0ca50cc7709a220b0561b85e53a48461fa88314e2afbd269d7da5e2b9931ccbd62fab5118a36618")

	contract := "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB"
	txBuilder2, _ := builder.NewTxBuilder(&xc.TokenAssetConfig{
		Contract:    contract,
		Decimals:    6,
		ChainConfig: &xc.ChainConfig{},
	})
	tnt, err = txBuilder2.NewTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tnt)
	xrpTx = tnt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.AmountValue.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.AmountValue.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
	require.Equal(t, xrpTx.Amount.AmountValue.Value, "12")
	encodeForSigningAsset := tnt.(*Tx).EncodeForSigning
	require.Equal(t, hex.EncodeToString(encodeForSigningAsset), "5354580012000022000000002400000000201b0000000061d4c44364c5bb0000000000000000000000000000464d540000000000cc3e26d717bb8b8ff8c9af7fcc18e5b5b3f504d368400000000000000a73008114f667b0ca50cc7709a220b0561b85e53a48461fa88314e2afbd269d7da5e2b9931ccbd62fab5118a36618")
}
