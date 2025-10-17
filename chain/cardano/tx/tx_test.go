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
	cfg := xc.NewChainConfig(xc.ADA).WithNet("preprod").WithDecimals(6)
	transferArgs, err := xcbuilder.NewTransferArgs(cfg.Base(), fromAddr, toAddr, amount)
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

	transaction, err := tx.NewTransfer(transferArgs, &transferInput)
	require.NoError(t, err)
	return transaction.(*tx.Tx)
}

func newStake(t *testing.T) *tx.Tx {
	args, _ := xcbuilder.NewStakeArgs(
		xc.ADA,
		xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
		xc.NewAmountBlockchainFromUint64(20),
		xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
		xcbuilder.OptionPublicKey(make([]byte, 32)),
	)

	stakeInput := tx_input.StakingInput{
		TxInput: tx_input.TxInput{
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
		},
	}

	stakeTx, err := tx.NewStake(args, &stakeInput)
	require.NoError(t, err)
	return stakeTx.(*tx.Tx)
}

func newUnstake(t *testing.T) *tx.Tx {
	args, _ := xcbuilder.NewStakeArgs(
		xc.ADA,
		xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
		xc.NewAmountBlockchainFromUint64(20),
		xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
		xcbuilder.OptionPublicKey(make([]byte, 32)),
	)

	unstakeInput := tx_input.UnstakingInput{
		TxInput: tx_input.TxInput{
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
		},
	}

	unstakeTx, err := tx.NewUnstake(args, &unstakeInput)
	require.NoError(t, err)
	return unstakeTx.(*tx.Tx)
}

func newWithdraw(t *testing.T) *tx.Tx {
	pk, _ := hex.DecodeString("f0bb6fd00a035b6b6ec18bbb2739265b80f319c0634333fe678928f40750cade")
	args, _ := xcbuilder.NewStakeArgs(
		xc.ADA,
		xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
		xc.NewAmountBlockchainFromUint64(20),
		xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
		xcbuilder.OptionPublicKey(pk),
	)

	withdrawInput := tx_input.WithdrawInput{
		TxInput: tx_input.TxInput{
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
		},
		RewardsAddress: "stake_test1upp33pdh0nppmxz8ma2def28nz8kju0yqrnmgfelcjf88fqd406dg",
	}

	withdrawTx, err := tx.NewWithdraw(args, &withdrawInput)
	require.NoError(t, err)
	return withdrawTx.(*tx.Tx)
}

func TestTxHash(t *testing.T) {
	tx := newTx(t)
	expectedHash := xc.TxHash("b9ef1b6a45fcf1584fcf77b388556c54a9209c4b68c3655168fb9ba676dedef3")
	require.Equal(t, expectedHash, tx.Hash())
}

func TestTxSighashes(t *testing.T) {
	tx := newTx(t)
	sighashes, err := tx.Sighashes()
	require.NotNil(t, sighashes)
	require.NoError(t, err)
	require.Equal(t, 1, len(sighashes))
	require.Equal(t, "b9ef1b6a45fcf1584fcf77b388556c54a9209c4b68c3655168fb9ba676dedef3", hex.EncodeToString(sighashes[0].Payload))
}

func TestStakingSighashes(t *testing.T) {
	stakeTx := newStake(t)
	sighashes, err := stakeTx.Sighashes()

	require.NoError(t, err)

	// Double signature is reqauired for stakes
	expectedPayload, _ := hex.DecodeString("b9ee1ef163a1812ed60bc7e828bf0c0de8f3aead8ce6e0e06eccaf62b845762a")
	require.Equal(t, []*xc.SignatureRequest{
		{
			Payload: expectedPayload,
		},
		{
			Payload: expectedPayload,
		},
	}, sighashes)
}

func TestUnstakingSighashes(t *testing.T) {
	args, _ := xcbuilder.NewStakeArgs(
		xc.ADA,
		xc.Address("addr_test1vzjddf57t45k7a04kpr65lakpjmx50pwy7v0eje3t73c02s5zecy5"),
		xc.NewAmountBlockchainFromUint64(20),
		xcbuilder.OptionValidator("dd4ed2b86a51c550cca3ba8cef374da75fe87d5d6664f562ac9d2bc9"),
		xcbuilder.OptionPublicKey(make([]byte, 32)),
	)

	unstakeInput := tx_input.UnstakingInput{
		TxInput: tx_input.TxInput{
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
		},
	}

	unstakeTx, err := tx.NewUnstake(args, &unstakeInput)
	require.NoError(t, err)
	sighashes, err := unstakeTx.Sighashes()

	require.NoError(t, err)

	// Double signature is reqauired for unstakes
	expectedPayload, _ := hex.DecodeString("bb36e9b4984e5d74355d45050940f5e28f1941b1f869403fba2bc838a366337f")
	require.Equal(t, []*xc.SignatureRequest{
		{
			Payload: expectedPayload,
		},
		{
			Payload: expectedPayload,
		},
	}, sighashes)
}

func TestWithdrawalSighashes(t *testing.T) {
	withdrawTx := newWithdraw(t)
	sighashes, err := withdrawTx.Sighashes()

	require.NoError(t, err)

	// Double signature is reqauired for withdrawals
	expectedPayload, _ := hex.DecodeString("3292140cdcda46a9eafcfafe27d9e795dd5032b82ef636a43d9f5b4caba7447e")
	require.Equal(t, []*xc.SignatureRequest{
		{
			Payload: expectedPayload,
		},
		{
			Payload: expectedPayload,
		},
	}, sighashes)
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
		{
			name: "StakeSingleSig",
			tx:   newStake(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				}},
			err: "invalid signature count",
		},
		{
			name: "ValidStakeSigs",
			tx:   newStake(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
			},
			err: "",
		},
		{
			name: "SingleUnstakeSig",
			tx:   newUnstake(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
			},
			err: "invalid signature count",
		},
		{
			name: "ValidUnstakeSigs",
			tx:   newUnstake(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
			},
			err: "",
		},
		{
			name: "SingleWithdrawalSig",
			tx:   newWithdraw(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
			},
			err: "invalid signature count",
		},
		{
			name: "ValidWithdrawalSigs",
			tx:   newWithdraw(t),
			sigs: []*xc.SignatureResponse{
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
				{
					Signature: xc.TxSignature("b9af112fa07e603c08827c2752eaf09ff0afc0726c8c5a3d923e743398e6a0c141b5476623faa88a451ae3fce2518c9502768d777e7c96d509cfbc62502d300a"),
				},
			},
			err: "",
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
