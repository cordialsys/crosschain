package tx_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/btree"
)

func newTx(t *testing.T) *tx.Tx {
	fromAddr := xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5")
	toAddr := xc.Address("addr_test1qrfp5xelv2mu7k8zyvwm0c8t5xm55wanwhtd4fgjgtf3ck0rplhn7x9jyhwqg70fwv0ujpmyumqk5td9e9hnsejtlxnq3yqf25")
	amount := xc.NewAmountBlockchainFromUint64(1_000_000)

	transferArgs, err := xcbuilder.NewTransferArgs(fromAddr, toAddr, amount)
	require.NoError(t, err)

	transferInput := tx_input.TxInput{
		Utxos: []types.Utxo{
			{
				Address: "addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5",
				Amounts: []types.Amount{
					{
						Unit:     "lovelace",
						Quantity: "5333004",
					},
				},
				TxHash: "72cfa181469b48402a50c6652d45c789897ae5025bb01f569a7bd01bffd12bc1",
				Index:  1,
			},
		},
		Slot: 90_751_416,
	}

	transaction, err := tx.NewTx(transferArgs, transferInput)
	require.NoError(t, err)
	return transaction.(*tx.Tx)
}

func TestTxHash(t *testing.T) {
	tx := newTx(t)
	expectedHash := xc.TxHash("53c1a2f0954b7827da2294a13242fc8cd6046ee346bdff8072e3db82335d1d86")
	require.Equal(t, expectedHash, tx.Hash())
}

func TestTxSighashes(t *testing.T) {
	tx := newTx(t)
	sighashes, err := tx.Sighashes()
	require.NotNil(t, sighashes)
	require.NoError(t, err)
	require.Equal(t, 1, len(sighashes))
	require.Equal(t, "53c1a2f0954b7827da2294a13242fc8cd6046ee346bdff8072e3db82335d1d86", hex.EncodeToString(sighashes[0].Payload))
}

func TestTxAddSignature(t *testing.T) {
	txWithWitness := newTx(t)
	txWithWitness.Witness = &tx.Witness{
		Keys: []*tx.VKeyWitness{
			{
				VKey:      []byte{},
				Signature: []byte{},
			},
		},
	}

	vectors := []struct {
		name string
		tx   *tx.Tx
		sigs []*xc.SignatureResponse
		err  string
	}{
		{
			name: "ValidSignature",
			tx:   newTx(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				}},
			err: "",
		},
		{
			name: "AlreadySigned",
			tx:   txWithWitness,
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				}},
			err: "tx already signed",
		},
		{
			name: "EmptySigs",
			tx:   newTx(t),
			sigs: []*xc.SignatureResponse{},
			err:  "no signatures provided",
		},
	}

	for _, vector := range vectors {
		t.Run(vector.name, func(t *testing.T) {
			err := vector.tx.SetSignatures(vector.sigs...)
			if vector.err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, vector.err)
			}

		})
	}

}

func TestCalcMinAdaValue(t *testing.T) {
	onePolicyNoAssetNames := btree.NewMap[tx.PolicyHash, tx.TokenNameHexToAmount](1)
	assetsToAmounts := btree.NewMap[tx.TokenName, uint64](1)
	assetsToAmounts.Set(tx.TokenName(""), 1_000_000)
	onePolicyNoAssetNames.Set(
		tx.PolicyHash("00000000000000000000000000000000000000000000000000000000"),
		tx.TokenNameHexToAmount{assetsToAmounts})

	onePolicyOne1LenName := btree.NewMap[tx.PolicyHash, tx.TokenNameHexToAmount](1)
	assetsToAmounts1 := btree.NewMap[tx.TokenName, uint64](1)
	assetsToAmounts1.Set(tx.TokenName("0"), 1_000_000)
	onePolicyOne1LenName.Set(
		tx.PolicyHash("00000000000000000000000000000000000000000000000000000000"),
		tx.TokenNameHexToAmount{assetsToAmounts1})

	onePolicyOne32LenName := btree.NewMap[tx.PolicyHash, tx.TokenNameHexToAmount](1)
	assetsToAmount32 := btree.NewMap[tx.TokenName, uint64](1)
	name32 := "01234567890123456789012345678901"
	assetsToAmount32.Set(tx.TokenName(name32), 1_000_000)
	onePolicyOne32LenName.Set(
		tx.PolicyHash("00000000000000000000000000000000000000000000000000000000"),
		tx.TokenNameHexToAmount{assetsToAmount32})

	vectors := []struct {
		Name            string
		PolicyToAmounts tx.PolicyIdToAmounts
		ExpectedSize    uint64
	}{
		{
			Name:            "OnePolicyNoAssetNames",
			PolicyToAmounts: tx.PolicyIdToAmounts{onePolicyNoAssetNames},
			ExpectedSize:    1_407_406,
		},
		{
			Name:            "OnePolicySingleLetterName",
			PolicyToAmounts: tx.PolicyIdToAmounts{onePolicyOne1LenName},
			ExpectedSize:    1_444_443,
		},
		{
			Name:            "OnePolicyOne32LenName",
			PolicyToAmounts: tx.PolicyIdToAmounts{onePolicyOne32LenName},
			ExpectedSize:    1_555_554,
		},
	}

	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			minAdaValue := tx.CalcMinAdaValue(&v.PolicyToAmounts)
			require.Equal(t, v.ExpectedSize, minAdaValue.Uint64())
		})
	}
}
