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
	now := uint64(time.Now().UnixNano())
	vectors := []testcase{
		{
			newInput: &TxInput{
				CreatedAtTime: now,
			},
			oldInput: &TxInput{
				CreatedAtTime: now - uint64((time.Hour * 5).Nanoseconds()),
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
				CreatedAtTime: now,
			},
			oldInput: &TxInput{
				CreatedAtTime: now - uint64((time.Hour * 25).Nanoseconds()),
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
