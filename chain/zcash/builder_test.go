package zcash_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/cordialsys/crosschain/chain/zcash"
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

	cfg := xc.NewChainConfig(xc.ZEC).WithNet("mainnet")

	txBuilder, err := zcash.NewTxBuilder(cfg.Base())
	if err != nil {
		t.Fatal(err)
	}
	from := xc.Address("t1g4xVgMHVsxZWxS6D3SLXNXEAicivXKiAS")
	to := xc.Address("t1PyjotZbtna7jhzpF4w35wNFX2GGRJFcXM")
	amount := xc.NewAmountBlockchainFromUint64(100000000)
	args, err := builder.NewTransferArgs(cfg.Base(), from, to, amount)
	require.NoError(t, err)

	tx, err := txBuilder.Transfer(args, input)
	require.NoError(t, err)

	// tx must be a bitcoin cash tx
	btcTx, ok := tx.(*zcash.Tx)
	require.True(t, ok)
	require.NotNil(t, btcTx)

	sighashes, err := btcTx.Sighashes()
	require.NoError(t, err)
	require.Equal(t, 1, len(sighashes))
	require.Equal(t,
		// will have to update if anything changes in ZEC
		"8ae25afd8e3e6373dce8814ac74097d949238bd0eac22a7229ed6a0c376d7498",
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
		// will have to update if anything changes in ZEC
		"0400008085202f8901000000000b0930060201000201000100ffffffff0200e1f505000000001976a91442f9c388bff9d1f180388e5f644cc62d3864c06888ac00e1f505000000001976a914f37872277cd9210d5506f372e840c3da3ae11cf988ac00000000000000000000000000000000000000",
		hex.EncodeToString(serialized),
	)
}
