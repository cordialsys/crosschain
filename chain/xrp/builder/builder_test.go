package builder_test

import (
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
	nt, err := txBuilder.NewNativeTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, nt)
	xrpTx := nt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.XRPAmount, "12")
}

func TestNewTokenTransfer(t *testing.T) {

	contract := "FMT-rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB"
	txBuilder, _ := builder.NewTxBuilder(&xc.TokenAssetConfig{
		Contract:    contract,
		Decimals:    16,
		ChainConfig: &xc.ChainConfig{},
	})
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &TxInput{}
	tt, err := txBuilder.NewTokenTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tt)
	xrpTx := tt.(*Tx).XRPTx
	require.Equal(t, xrpTx.Amount.TokenAmount.Value, "12")
	require.Equal(t, xrpTx.Amount.TokenAmount.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.TokenAmount.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
}

func TestNewTransfer(t *testing.T) {

	txBuilder1, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	from := xc.Address("rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	to := xc.Address("rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	amount := xc.NewAmountBlockchainFromUint64(12)
	input := &TxInput{}
	tnt, err := txBuilder1.NewTransfer(from, to, amount, input)
	require.NoError(t, err)
	require.NotNil(t, tnt)
	xrpTx := tnt.(*Tx).XRPTx
	require.Equal(t, string(xrpTx.Account), "rPT1Sjq2YGrBMTttX4GZHjKu9dyfzbpAYe")
	require.Equal(t, string(xrpTx.Destination), "rMCcNuTcajgw7YTgBy1sys3b89QqjUrMpH")
	require.Equal(t, xrpTx.Amount.XRPAmount, "12")

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
	require.Equal(t, xrpTx.Amount.TokenAmount.Currency, "FMT")
	require.Equal(t, xrpTx.Amount.TokenAmount.Issuer, "rKcAJWccYkYr7Mh2ZYmZFyLzhZD23DvTvB")
	require.Equal(t, xrpTx.Amount.TokenAmount.Value, "12")
}
