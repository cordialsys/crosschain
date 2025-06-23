package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/template/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestSafeFromDoubleSpend(t *testing.T) {

	newInput := &TxInput{}
	oldInput1 := &TxInput{}
	// Defaults are false but each chain has conditions
	require.False(t, newInput.SafeFromDoubleSend(oldInput1))
	require.False(t, newInput.IndependentOf(oldInput1))
}

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		oldInput xc.TxInput
		newInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			oldInput:        &TxInput{},
			newInput:        &TxInput{},
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{},
			// check no old input
			oldInput:        nil,
			independent:     false,
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
