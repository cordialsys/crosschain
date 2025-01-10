package tx_input_test

import (
	"github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/test-go/testify/require"
	"testing"
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
