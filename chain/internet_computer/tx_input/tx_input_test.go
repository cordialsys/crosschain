package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/internet_computer/tx_input"
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
	now := time.Now()
	vectors := []testcase{
		{
			newInput: &TxInput{
				CreateTime: now.Unix(),
			},
			oldInput: &TxInput{
				CreateTime: now.Add(tx_input.SafetyTimeoutMargin / 2).Unix(),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{},
			// check no old input
			oldInput:        nil,
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				CreateTime: now.Unix(),
			},
			oldInput: &TxInput{
				CreateTime: now.Add(-tx_input.SafetyTimeoutMargin - 1*time.Minute).Unix(),
			},
			independent:     true,
			doubleSpendSafe: true,
		},
	}
	for i, v := range vectors {
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
	}
}
