package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hyperliquid/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		oldInput xc.TxInput
		newInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	vectors := []testcase{
		{
			oldInput: &TxInput{
				TransactionTime: time.UnixMilli(1758098790757),
			},
			newInput: &TxInput{
				TransactionTime: time.UnixMilli(1758098790757),
			},
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			oldInput: &TxInput{
				TransactionTime: time.UnixMilli(1757754534000),
			},
			newInput: &TxInput{
				TransactionTime: time.UnixMilli(1758098790757), // old + 3 days
			},
			independent:     true,
			doubleSpendSafe: true,
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
