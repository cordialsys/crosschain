package tx_input_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cardano/client/types"
	"github.com/cordialsys/crosschain/chain/cardano/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInputConflicts(t *testing.T) {
	type testcase struct {
		name     string
		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			name: "IndependentNotSafeFromDoubleSpend",
			newInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash1",
						Index:  0,
					},
				},
			},
			oldInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash2h",
						Index:  0,
					},
				},
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name: "IndependentTheSameHash",
			newInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash1",
						Index:  0,
					},
				},
			},
			oldInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash1",
						Index:  1,
					},
				},
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name: "SafeFromDoubleSpend",
			newInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash1",
						Index:  0,
					},
				},
			},
			oldInput: &TxInput{
				Utxos: []types.Utxo{
					{
						TxHash: "hash1",
						Index:  0,
					},
				},
			},
			independent:     false,
			doubleSpendSafe: true,
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			require.Equal(t,
				v.newInput.IndependentOf(v.oldInput),
				v.independent,
				"IndependentOf",
			)
			require.Equal(t,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				v.doubleSpendSafe,
				"SafeFromDoubleSend",
			)
		})
	}
}
