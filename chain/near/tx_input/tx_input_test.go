package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/near/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInput(t *testing.T) {

	type testcase struct {
		oldInput xc.TxInput
		newInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			oldInput: &TxInput{
				Nonce: 0,
			},
			newInput: &TxInput{
				Nonce: 0,
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				Nonce: 1,
			},
			// check no old input
			oldInput: &TxInput{
				Nonce: 2,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("testcase_%d", i), func(t *testing.T) {
			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			t.Logf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
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
