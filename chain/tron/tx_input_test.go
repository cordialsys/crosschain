package tron_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/tron/txinput"
	"github.com/stretchr/testify/require"
)

func TestTxInputConflicts(t *testing.T) {
	testcases := []struct {
		name            string
		newInput        xc.TxInput
		oldInput        xc.TxInput
		independent     bool
		doubleSpendSafe bool
	}{
		{
			name: "expired old input",
			newInput: &txinput.TxInput{
				Timestamp:  1000 + 60,
				Expiration: 2000,
			},
			oldInput: &txinput.TxInput{
				Timestamp:  100,
				Expiration: 999,
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			name: "overlapping old input",
			newInput: &txinput.TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: &txinput.TxInput{
				Timestamp:  100,
				Expiration: 2001,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name: "old input contains new input",
			newInput: &txinput.TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: &txinput.TxInput{
				Timestamp:  0,
				Expiration: 1000000,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name: "nil old input",
			newInput: &txinput.TxInput{
				Timestamp:  1000,
				Expiration: 2000,
			},
			oldInput: nil,
			// tron is always independent
			independent:     true,
			doubleSpendSafe: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(
				t,
				tc.independent,
				tc.newInput.IndependentOf(tc.oldInput),
				"IndependentOf",
			)
			require.Equal(
				t,
				tc.doubleSpendSafe,
				tc.newInput.SafeFromDoubleSend(tc.oldInput),
				"SafeFromDoubleSend",
			)
		})
	}
}
