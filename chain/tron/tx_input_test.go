package tron

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
			newInput: &TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: &TxInput{
				Timestamp:  100,
				Expiration: 999,
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: &TxInput{
				Timestamp:  100,
				Expiration: 2001,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: &TxInput{
				Timestamp:  0,
				Expiration: 1000000,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: nil,
			// tron is always independent
			independent:     true,
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
