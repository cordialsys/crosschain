package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	"github.com/test-go/testify/require"
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
			newInput:        &TxInput{Sequence: 10},
			oldInput:        &TxInput{Sequence: 10},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			// may have different accounts
			newInput:        &TxInput{Sequence: 10, AccountNumber: 10},
			oldInput:        &TxInput{Sequence: 10, AccountNumber: 11},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{Sequence: 10},
			oldInput:        &TxInput{Sequence: 11},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{Sequence: 10},
			oldInput:        &TxInput{Sequence: 9},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{Sequence: 10},
			oldInput: nil,
			// default false, not always independent
			independent:     false,
			doubleSpendSafe: false,
		},
	}
	for i, v := range vectors {
		newBz, _ := json.Marshal(v.newInput)
		oldBz, _ := json.Marshal(v.oldInput)
		fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
		fmt.Println()
		require.Equal(
			t,
			v.newInput.IndependentOf(v.oldInput),
			v.independent,
			"IndependentOf",
		)
		require.Equal(
			t,
			v.newInput.SafeFromDoubleSend(v.oldInput),
			v.doubleSpendSafe,
			"SafeFromDoubleSend",
		)
	}
}

func TestTxInputGasMultiplier(t *testing.T) {
	type testcase struct {
		input      *TxInput
		multiplier string
		result     uint64
		err        bool
	}
	vectors := []testcase{
		{
			input:      &TxInput{GasPrice: 100.0},
			multiplier: "1.5",
			result:     150,
		},
		{
			input:      &TxInput{GasPrice: 100.0},
			multiplier: "1",
			result:     100,
		},
		{
			input:      &TxInput{GasPrice: 100.0},
			multiplier: "abc",
			err:        true,
		},
	}
	for i, v := range vectors {
		desc := fmt.Sprintf("testcase %d: mult = %s", i, v.multiplier)
		err := v.input.SetGasFeePriority(xc.GasFeePriority(v.multiplier))
		if v.err {
			require.Error(t, err, desc)
		} else {
			require.Equal(t, v.result, uint64(v.input.GasPrice), desc)
		}
	}
}
