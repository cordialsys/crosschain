package tx_input_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/xlm/tx_input"
	"github.com/cordialsys/crosschain/factory/drivers"
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

func TestTxInputUnmarshal(t *testing.T) {
	// inputBz := "{\"asset\":\"XLM\",\"balance\":\"250000000\",\"chain\":\"XLM\",\"extra\":{},\"from\":\"GAUHHCJGQMRDNQAAWX22PF6YGZ27JFVISJHA7YYN5JRTO3RZ5YZVICQ7\",\"input\":{\"MaxFee\":10000000,\"MinLedgerSequence\":2035163,\"Passphrase\":\"Test SDF Network ; September 2015\",\"Sequence\":8740778138402817,\"TransactionActiveTime\":7200000000000,\"sequence\":\"8740778138402817\",\"type\":\"xlm\"},\"public_key\":\"28738926832236c000b5f5a797d83675f496a8924e0fe30dea63376e39ee3354\",\"raw_tx_input\":,\"to\":\"GCGJ3NRXDDMU67YVBGC76XSKNGUKTTI6M6AKKDB324OOHJYMYRFXLH76\"}"
	inputBz := "{\"MaxFee\":10000000,\"MinLedgerSequence\":2035163,\"Passphrase\":\"Test SDF Network ; September 2015\",\"Sequence\":8740778138402817,\"TransactionActiveTime\":7200000000000,\"sequence\":\"8740778138402817\",\"type\":\"xlm\"}"

	input, err := drivers.UnmarshalTxInput([]byte(inputBz))
	require.NoError(t, err)
	require.NotNil(t, input)
}
