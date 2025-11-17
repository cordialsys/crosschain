package txinfo

import (
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/stretchr/testify/require"
)

func TestTxInfoFees(t *testing.T) {

	chainCfg := xc.NewChainConfig(xc.ETH)
	chainCfg.Confirmations.Final = 3

	tx := NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		3,
		nil,
	)

	// adding simple transfers should never add a fee
	for i := 0; i < 10; i++ {
		from := xc.Address(fmt.Sprintf("from-%d", i))
		to := xc.Address(fmt.Sprintf("to-%d", i))

		tx.AddSimpleTransfer(from, to, "", xc.NewAmountBlockchainFromUint64(10), nil, "")
		require.Len(t, tx.CalculateFees(), 0)
	}

	// manually add a fee
	tf := NewMovement(tx.XChain, "")
	tf.AddSource("feepayer", xc.NewAmountBlockchainFromUint64(55), nil)
	tx.AddMovement(tf)
	require.Len(t, tx.CalculateFees(), 1)
	require.Equal(t, "55", tx.CalculateFees()[0].Balance.String())

	// add a fee via helper
	tx.AddFee("feepayer", "", xc.NewAmountBlockchainFromUint64(65), nil)
	require.Len(t, tx.CalculateFees(), 1)
	require.Equal(t, "120", tx.CalculateFees()[0].Balance.String())

	// add a fee of new asset via helper
	tx.AddFee("feepayer", "USDC", xc.NewAmountBlockchainFromUint64(65), nil)
	require.Len(t, tx.CalculateFees(), 2)
	require.Equal(t, "120", tx.CalculateFees()[0].Balance.String())
	require.Equal(t, "65", tx.CalculateFees()[1].Balance.String())

	tx.AddSimpleTransfer("a", "b", "", xc.NewAmountBlockchainFromUint64(0), nil, "memo")
	require.Equal(t, "memo", tx.Movements[len(tx.Movements)-1].Memo)

	require.Equal(t, true, tx.Final)
	require.Equal(t, Succeeded, tx.State)

}

func TestTxInfoMultiLegFees(t *testing.T) {

	chainCfg := xc.NewChainConfig(xc.BTC)
	chainCfg.Confirmations.Final = 3
	tx := NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		3,
		nil,
	)

	tf := NewMovement(tx.XChain, "")
	for i := 0; i < 10; i++ {
		tf.AddSource("sender", xc.NewAmountBlockchainFromUint64(100), nil)
	}
	for i := 0; i < 8; i++ {
		tf.AddDestination("sender", xc.NewAmountBlockchainFromUint64(100), nil)
	}
	tx.AddMovement(tf)
	require.Len(t, tx.CalculateFees(), 1)
	// 1000 - 800
	require.Equal(t, "200", tx.CalculateFees()[0].Balance.String())
	require.EqualValues(t, "BTC", tx.CalculateFees()[0].Contract)
}

// This is like `TestTxInfoMultiLegFees`, but we add every balance change as an
// independent transfer, and test we can coalesce them into 1 transfer again.
func TestTxInfoMultiLegCoalesce(t *testing.T) {
	chainCfg := xc.NewChainConfig(xc.BTC)
	chainCfg.Confirmations.Final = 3
	tx := NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		3,
		nil,
	)
	for i := 0; i < 10; i++ {
		tf := NewMovement(tx.XChain, "")
		tf.AddSource("sender", xc.NewAmountBlockchainFromUint64(100), nil)
		tx.AddMovement(tf)
	}
	for i := 0; i < 8; i++ {
		tf := NewMovement(tx.XChain, "")
		tf.AddDestination("sender", xc.NewAmountBlockchainFromUint64(100), nil)
		tx.AddMovement(tf)
	}

	// Fee calculation should work fine
	require.Len(t, tx.CalculateFees(), 1)
	require.Equal(t, "200", tx.CalculateFees()[0].Balance.String())
	require.EqualValues(t, "BTC", tx.CalculateFees()[0].Contract)
	require.Len(t, tx.Movements, 18)

	// Coalesce should simplify into 1 transfer that's equivilent.
	tx.Coalesece()
	require.Len(t, tx.Movements, 1)
	require.Equal(t, "200", tx.CalculateFees()[0].Balance.String())
	require.EqualValues(t, "BTC", tx.CalculateFees()[0].Contract)
}

func TestTxInfoState(t *testing.T) {
	chainCfg := xc.NewChainConfig(xc.BTC)
	chainCfg.Confirmations.Final = 3

	// succeeded, final
	tx := NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		3,
		nil,
	)
	require.True(t, tx.Final, "final")
	require.Equal(t, Succeeded, tx.State)

	// succeeded, not final
	tx = NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		2,
		nil,
	)
	require.False(t, tx.Final, "final")
	require.Equal(t, Succeeded, tx.State)

	// failed
	errMsg := "err"
	tx = NewTxInfo(
		NewBlock(chainCfg.Chain, 1, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		2,
		&errMsg,
	)
	require.False(t, tx.Final, "final")
	require.Equal(t, Failed, tx.State)

	// mining
	tx = NewTxInfo(
		NewBlock(chainCfg.Chain, 0, "1234", time.Unix(1, 0)),
		chainCfg,
		"0x1234",
		0,
		nil,
	)
	require.False(t, tx.Final, "final")
	require.Equal(t, Mining, tx.State)
}
