package bitcoin_cash_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/test-go/testify/require"
)

func TestTransfer(t *testing.T) {
	input := tx_input.NewTxInput()
	input.UnspentOutputs = []tx_input.Output{
		{
			Value:        xc.NewAmountBlockchainFromUint64(200000000),
			PubKeyScript: []byte{},
		},
	}

	cfg := xc.NewChainConfig(xc.BCH).WithNet("mainnet")

	txBuilder, err := bitcoin_cash.NewTxBuilder(cfg.Base())
	if err != nil {
		t.Fatal(err)
	}
	from := xc.Address("qrgn26exqv2lly6hsl887k8tqjatgr0lwg76fgk2u4")
	to := xc.Address("qpehj6d2ddf2ptt4fff5deu2wj7dhsr2rg0766dzsc")
	amount := xc.NewAmountBlockchainFromUint64(100000000)
	args, err := builder.NewTransferArgs(from, to, amount)
	require.NoError(t, err)

	tx, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)

	// tx must be a bitcoin cash tx
	btcTx, ok := tx.(*bitcoin_cash.Tx)
	require.True(t, ok)
	require.NotNil(t, btcTx)
}
