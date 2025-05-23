package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xrp/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestSafeFromDoubleSpend(t *testing.T) {

	newInput := &TxInput{}
	oldInput1 := &TxInput{}
	// Defaults are false but each chain has conditions
	require.True(t, newInput.SafeFromDoubleSend(oldInput1))
	require.False(t, newInput.IndependentOf(oldInput1))
}

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}

	vectors := []testcase{
		{
			newInput: &TxInput{
				V2Sequence:           22811103,
				V2LastLedgerSequence: 90986722,
			},
			oldInput: &TxInput{
				V2Sequence:           22811103,
				V2LastLedgerSequence: 90986722,
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				V2Sequence:           22811103,
				V2LastLedgerSequence: 90986722,
			},
			oldInput: &TxInput{
				V2Sequence:           22811104,
				V2LastLedgerSequence: 90986722,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{},
			oldInput:        &TxInput{},
			independent:     false,
			doubleSpendSafe: true,
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
		newBz, _ := json.Marshal(v.newInput)
		oldBz, _ := json.Marshal(v.oldInput)
		fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
		fmt.Println()
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
	}
}
