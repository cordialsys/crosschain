package tx_input_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/test-go/testify/require"
)

type TxInput = tx_input.TxInput

func TestSafeFromDoubleSpend(t *testing.T) {
	newInput := &TxInput{}
	oldInput := &TxInput{}

	require.True(t, newInput.SafeFromDoubleSend(oldInput))
	require.False(t, newInput.IndependentOf(oldInput))

	newInput = &TxInput{
		Sequence: 1,
	}
	oldInput = &TxInput{
		Sequence: 1,
	}
	require.True(t, newInput.SafeFromDoubleSend(oldInput))
	require.False(t, newInput.IndependentOf(oldInput))

	newInput = &TxInput{
		Sequence: 1,
	}
	oldInput = &TxInput{
		Sequence: 2,
	}
	require.False(t, newInput.SafeFromDoubleSend(oldInput))
	require.True(t, newInput.IndependentOf(oldInput))

	newInput = &TxInput{
		Sequence: 1,
	}
	require.False(t, newInput.SafeFromDoubleSend(oldInput))
	require.False(t, newInput.IndependentOf(nil))
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
			input:      &TxInput{MaxFee: 100},
			multiplier: "1.5",
			result:     150,
		},
	}
	for i, v := range vectors {
		desc := fmt.Sprintf("testcase %d: mult = %s", i, v.multiplier)
		err := v.input.SetGasFeePriority(xc.GasFeePriority(v.multiplier))
		if v.err {
			require.Error(t, err, desc)
		} else {
			require.Equal(t, v.result, uint64(v.input.MaxFee), desc)
		}
	}
}
