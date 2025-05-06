package builder_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/builder"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestNewTxBuilder(t *testing.T) {
	builder1, err := builder.NewTxBuilder(xc.NewChainConfig(xc.ADA).Base())
	require.NotNil(t, builder1)
	require.NoError(t, err)
}

func TestNewNativeTransfer(t *testing.T) {
	fromAddr := xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5")

	toAddr := xc.Address("addr_test1qrfp5xelv2mu7k8zyvwm0c8t5xm55wanwhtd4fgjgtf3ck0rplhn7x9jyhwqg70fwv0ujpmyumqk5td9e9hnsejtlxnq3yqf25")
	_, expectedToKey, err := tx.DecodeToBase256(string(toAddr))

	amount := xc.NewAmountBlockchainFromUint64(1_000_000)
	transferArgs, err := xcbuilder.NewTransferArgs(fromAddr, toAddr, amount)
	require.NoError(t, err)

	expectedUtxoHash := "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1"
	expectedUtxoIndex := uint16(1)
	expectedUtxoAmount := "5333004"
	xcInputAmount := xc.NewAmountBlockchainFromStr(expectedUtxoAmount)
	transferInput := tx_input.TxInput{
		Utxos: []types.Utxo{
			{
				Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
				Amounts: []types.Amount{
					{
						Unit:     "lovelace",
						Quantity: expectedUtxoAmount,
					},
				},
				TxHash: expectedUtxoHash,
				Index:  expectedUtxoIndex,
			},
		},
		Slot: 90_751_416,
	}

	cfg := xc.NewChainConfig(xc.ADA).WithNet("preprod").WithDecimals(6)
	builder, err := builder.NewTxBuilder(cfg.Base())
	require.NoError(t, err)

	transfer, err := builder.Transfer(transferArgs, &transferInput)
	require.NoError(t, err)

	cardanoTx, ok := transfer.(*tx.Tx)
	require.True(t, ok)

	require.NoError(t, err)
	require.Equal(t, expectedUtxoHash, hex.EncodeToString(cardanoTx.Body.Inputs[0].TxHash))
	require.Equal(t, expectedUtxoIndex, cardanoTx.Body.Inputs[0].Index)

	// Expect 2 outputs: to receiver, and change
	output0 := cardanoTx.Body.Outputs[0]
	output1 := cardanoTx.Body.Outputs[1]
	require.Equal(t, 2, len(cardanoTx.Body.Outputs))
	require.Equal(t, expectedToKey, output0.Address)

	output0Amount := xc.NewAmountBlockchainFromUint64(output0.TokenAmounts.NativeAmount)
	xcInputAmount = xcInputAmount.Sub(&output0Amount)
	output1Amount := xc.NewAmountBlockchainFromUint64(output1.TokenAmounts.NativeAmount)
	xcInputAmount = xcInputAmount.Sub(&output1Amount)
	feeAmount := xc.NewAmountBlockchainFromUint64(cardanoTx.Body.Fee)
	xcInputAmount = xcInputAmount.Sub(&feeAmount)

	require.Zero(t, xcInputAmount.Uint64())
}
