package client_test

import (
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/client"
	"github.com/stretchr/testify/require"
)

func TestTxInfoFees(t *testing.T) {

	tx := client.NewTxInfo(
		client.NewBlock(1, "1234", time.Unix(1, 0)),
		xc.ETH,
		"0x1234",
		3,
		nil,
	)

	// adding simple transfers should never add a fee
	for i := 0; i < 10; i++ {
		var from xc.Address = xc.Address(fmt.Sprintf("from-%d", i))
		var to xc.Address = xc.Address(fmt.Sprintf("to-%d", i))

		tx.AddSimpleTransfer(from, to, "", xc.NewAmountBlockchainFromUint64(10), nil, "")
		require.Len(t, tx.CalculateFees(), 0)
	}

	// manually add a fee
	tf := client.NewTransfer(tx.Chain)
	tf.AddSource("feepayer", "", xc.NewAmountBlockchainFromUint64(55), nil)
	tx.AddTransfer(tf)
	require.Len(t, tx.CalculateFees(), 1)
	require.Equal(t, "55", tx.CalculateFees()[0].Balance.String())

	// add a fee via helper
	tx.AddFee("feepayer", "", xc.NewAmountBlockchainFromUint64(65), nil)
	require.Len(t, tx.CalculateFees(), 1)
	require.Equal(t, "120", tx.CalculateFees()[0].Balance.String())

	// add a fee of new asset via helper
	tx.AddFee("feepayer", "USDC", xc.NewAmountBlockchainFromUint64(65), nil)
	require.Len(t, tx.CalculateFees(), 2)
	require.Equal(t, "65", tx.CalculateFees()[0].Balance.String())
	require.Equal(t, "120", tx.CalculateFees()[1].Balance.String())

	tx.AddSimpleTransfer("a", "b", "", xc.NewAmountBlockchainFromUint64(0), nil, "memo")
	require.Equal(t, "memo", tx.Transfers[len(tx.Transfers)-1].Memo)

}

func TestTxInfoMultiLegFees(t *testing.T) {
	tx := client.NewTxInfo(
		client.NewBlock(1, "1234", time.Unix(1, 0)),
		xc.BTC,
		"0x1234",
		3,
		nil,
	)
	tf := client.NewTransfer(tx.Chain)
	for i := 0; i < 10; i++ {
		tf.AddSource("sender", "", xc.NewAmountBlockchainFromUint64(100), nil)
	}
	for i := 0; i < 8; i++ {
		tf.AddDestination("sender", "", xc.NewAmountBlockchainFromUint64(100), nil)
	}
	tx.AddTransfer(tf)
	require.Len(t, tx.CalculateFees(), 1)
	// 1000 - 800
	require.Equal(t, "200", tx.CalculateFees()[0].Balance.String())
	require.EqualValues(t, "BTC", tx.CalculateFees()[0].Contract)
}
