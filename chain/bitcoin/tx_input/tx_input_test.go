package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/stretchr/testify/require"
)

func newPoint(hash []byte, index int) tx_input.Outpoint {
	return tx_input.Outpoint{
		Hash:  hash,
		Index: uint32(index),
	}
}
func newInput(points ...tx_input.Outpoint) *tx_input.TxInput {
	input := tx_input.TxInput{}
	for _, p := range points {
		input.UnspentOutputs = append(input.UnspentOutputs, tx_input.Output{
			Outpoint: p,
		})
	}
	return &input
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
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{11}, 11),
				newPoint([]byte{13}, 13),
			),
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: newInput(
				newPoint([]byte{10}, 11),
				newPoint([]byte{12}, 13),
			),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: newInput(
				newPoint([]byte{11}, 11),
				newPoint([]byte{13}, 13),
			),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: newInput(
				newPoint([]byte{11}, 11),
			),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: nil,
			// must be false for both, not always independent
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
			t,
			v.newInput.IndependentOf(v.oldInput),
			v.independent,
			"IndependentOf",
		)
		require.Equal(
			t,
			v.newInput.SafeFromDoubleSend(v.oldInput),
			v.doubleSpendSafe,
			"SafeFromDoubleSend",
		)
	}
}

func TestTxInputGasMultiplier(t *testing.T) {
	type testcase struct {
		input      *tx_input.TxInput
		multiplier string
		result     uint64
		err        bool
	}
	vectors := []testcase{
		{
			input:      &tx_input.TxInput{GasPricePerByte: xc.NewAmountBlockchainFromUint64(100)},
			multiplier: "1.5",
			result:     150,
		},
		{
			input:      &tx_input.TxInput{GasPricePerByte: xc.NewAmountBlockchainFromUint64(100)},
			multiplier: "1",
			result:     100,
		},
		{
			input:      &tx_input.TxInput{GasPricePerByte: xc.NewAmountBlockchainFromUint64(100)},
			multiplier: "abc",
			err:        true,
		},
	}
	for i, v := range vectors {
		desc := fmt.Sprintf("testcase %d: mult = %s", i, v.multiplier)
		err := v.input.SetGasFeePriority(xc.GasFeePriority(v.multiplier))
		if v.err {
			require.Error(t, err, desc)
		} else {
			require.Equal(t, v.result, v.input.GasPricePerByte.Uint64(), desc)
		}
	}
}
