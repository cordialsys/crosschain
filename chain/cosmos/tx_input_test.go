package cosmos

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestTxInputConflicts() {
	require := s.Require()
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
			v.newInput.IndependentOf(v.oldInput),
			v.independent,
			"IndependentOf",
		)
		require.Equal(
			v.newInput.SafeFromDoubleSend(v.oldInput),
			v.doubleSpendSafe,
			"SafeFromDoubleSend",
		)
	}
}
