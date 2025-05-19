package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/kaspa/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			newInput:        &TxInput{},
			oldInput:        &TxInput{},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{},
			// check no old input
			oldInput:        nil,
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				Utxos: []tx_input.Utxo{
					{
						TransactionId: "123",
						Index:         0,
					},
				},
			},
			oldInput: &TxInput{
				Utxos: []tx_input.Utxo{
					{
						TransactionId: "123",
						Index:         0,
					},
				},
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				Utxos: []tx_input.Utxo{
					{
						TransactionId: "1234",
						Index:         1,
					},
				},
			},
			oldInput: &TxInput{
				Utxos: []tx_input.Utxo{
					{
						TransactionId: "123",
						Index:         0,
					},
				},
			},
			independent:     true,
			doubleSpendSafe: false,
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
			fmt.Println()
			require.Equal(t,
				v.independent,
				v.newInput.IndependentOf(v.oldInput),
				"IndependentOf",
			)
			require.Equal(t,
				v.doubleSpendSafe,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				"SafeFromDoubleSend",
			)
		})
	}
}
