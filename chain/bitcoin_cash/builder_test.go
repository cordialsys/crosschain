package bitcoin_cash_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/bitcoin_cash"
	"github.com/stretchr/testify/require"
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

	sighashes, err := btcTx.Sighashes()
	require.NoError(t, err)
	require.Equal(t, 1, len(sighashes))
	require.Equal(t,
		// will have to update if anything changes in BCH
		"b2e256dc39726ed6b0b5f85f393187eefcf9bcbf2d27ddc0695920d320ce061b",
		hex.EncodeToString(sighashes[0].Payload),
	)

	signature := make([]byte, 64)
	err = btcTx.SetSignatures(&xc.SignatureResponse{
		Signature: signature,
	})
	require.NoError(t, err)

	serialized, err := btcTx.Serialize()
	require.NoError(t, err)

	require.Equal(t,
		// will have to update if anything changes in BCH
		"02000000010000000000000000000000000000000000000000000000000000000000000000000000000b0930060201000201004100ffffffff0200e1f505000000001976a914737969aa6b52a0ad754a5346e78a74bcdbc06a1a88ac00e1f505000000001976a914d1356b260315ff935787ce7f58eb04bab40dff7288ac00000000",
		hex.EncodeToString(serialized),
	)
}
