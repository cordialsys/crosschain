package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	"github.com/stretchr/testify/require"
)

var amount = xc.NewAmountBlockchainFromUint64

func newOutput(amount uint64) tx_input.Output {
	return tx_input.Output{
		Value: xc.NewAmountBlockchainFromUint64(amount),
	}
}

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

func newMultiInput(points ...tx_input.Outpoint) *tx_input.MultiTransferInput {
	input := tx_input.MultiTransferInput{}
	for _, p := range points {
		input.Inputs = append(input.Inputs, tx_input.TxInput{
			UnspentOutputs: []tx_input.Output{
				{
					Outpoint: p,
				},
			},
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
		{
			newInput: newMultiInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: nil,
			// must be false for both, not always independent
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			// verify we can handle tx-input vs multi-tx-input independence
			newInput: newInput(
				newPoint([]byte{10}, 10),
				newPoint([]byte{12}, 12),
			),
			oldInput: newMultiInput(
				newPoint([]byte{10}, 11),
				newPoint([]byte{12}, 13),
			),
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			// verify we can handle multi-tx-input vs tx-input independence
			newInput: newMultiInput(
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
			// verify we can handle multi-tx-input vs tx-input double-spend
			newInput: newMultiInput(
				newPoint([]byte{10}, 10), //dup
				newPoint([]byte{12}, 12),
			),
			oldInput: newInput(
				newPoint([]byte{10}, 10), //dup
				newPoint([]byte{12}, 13),
			),
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			// verify we can handle tx-input vs multi-tx-input double-spend
			newInput: newInput(
				newPoint([]byte{10}, 10), //dup
				newPoint([]byte{12}, 12),
			),
			oldInput: newMultiInput(
				newPoint([]byte{10}, 10), //dup
				newPoint([]byte{12}, 13),
			),
			independent:     false,
			doubleSpendSafe: true,
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("testcase %d", i), func(t *testing.T) {
			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
			fmt.Println()
			require.Equal(
				t,
				v.independent,
				v.newInput.IndependentOf(v.oldInput),
				"IndependentOf",
			)
			require.Equal(
				t,
				v.doubleSpendSafe,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				"SafeFromDoubleSend",
			)
		})
	}
}

func TestTxInputGasMultiplier(t *testing.T) {
	type testcase struct {
		input              *tx_input.TxInput
		multiplier         string
		multipliedGasPrice string
		totalFee           string
		err                bool
		legacy             bool
	}
	vectors := []testcase{
		{
			input:              &tx_input.TxInput{GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(100), EstimatedSizePerSpentUtxo: 10, UnspentOutputs: []tx_input.Output{{}}},
			multiplier:         "1.5",
			multipliedGasPrice: "150",
			totalFee:           "1500",
		},
		{
			// Legacy (using bigint gas price)
			input:              &tx_input.TxInput{XGasPricePerByte: xc.NewAmountBlockchainFromUint64(100), EstimatedSizePerSpentUtxo: 10, UnspentOutputs: []tx_input.Output{{}}},
			legacy:             true,
			multiplier:         "1.5",
			multipliedGasPrice: "150",
			totalFee:           "1500",
		},
		{
			input:              &tx_input.TxInput{GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(100), EstimatedSizePerSpentUtxo: 10, UnspentOutputs: []tx_input.Output{{}}},
			multiplier:         "1",
			multipliedGasPrice: "100",
			totalFee:           "1000",
		},
		{
			// Gas price less than 1
			input:              &tx_input.TxInput{GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(0.1), EstimatedSizePerSpentUtxo: 100, UnspentOutputs: []tx_input.Output{{}}},
			multiplier:         "1",
			multipliedGasPrice: "0.1",
			totalFee:           "10",
		},
		{
			// Gas price less than 1 with multiplier
			input:              &tx_input.TxInput{GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(0.1), EstimatedSizePerSpentUtxo: 100, UnspentOutputs: []tx_input.Output{{}}},
			multiplier:         "1.5",
			multipliedGasPrice: "0.15",
			totalFee:           "15",
		},
		{
			input:      &tx_input.TxInput{GasPricePerByteV2: xc.NewAmountHumanReadableFromFloat(100)},
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
			if v.legacy {
				require.Equal(t, v.multipliedGasPrice, v.input.XGasPricePerByte.String(), desc)
			} else {
				require.Equal(t, v.multipliedGasPrice, v.input.GasPricePerByteV2.String(), desc)
			}

			maxFee, _ := v.input.GetFeeLimit()
			require.Equal(t, v.totalFee, maxFee.String(), desc)
		}

	}
}

func TestFilterForMinUtxoSet(t *testing.T) {
	type testcase struct {
		name           string
		unspentOutputs []tx_input.Output
		targetAmount   xc.AmountBlockchain
		minTotalUtxo   int
		expected       []tx_input.Output
		expectedTotal  xc.AmountBlockchain
	}

	vectors := []testcase{
		{
			name:           "empty inputs",
			unspentOutputs: []tx_input.Output{},
			targetAmount:   amount(100),
			minTotalUtxo:   2,
			expected:       []tx_input.Output{},
			expectedTotal:  amount(0),
		},
		{
			name: "single utxo sufficient",
			unspentOutputs: []tx_input.Output{
				newOutput(1000),
			},
			targetAmount: amount(500),
			minTotalUtxo: 2,
			expected: []tx_input.Output{
				newOutput(1000),
			},
			expectedTotal: amount(1000),
		},
		{
			name: "multiple utxos, need all to reach target",
			unspentOutputs: []tx_input.Output{
				newOutput(100),
				newOutput(200),
				newOutput(300),
			},
			targetAmount: amount(550),
			minTotalUtxo: 1,
			expected: []tx_input.Output{
				newOutput(300),
				newOutput(200),
				newOutput(100),
			},
			expectedTotal: amount(600),
		},
		{
			name: "multiple utxos, need minExtraUtxo",
			unspentOutputs: []tx_input.Output{
				newOutput(1000),
				newOutput(100),
				newOutput(50),
				newOutput(25),
			},
			targetAmount: amount(500),
			minTotalUtxo: 3,
			expected: []tx_input.Output{
				newOutput(1000),
				newOutput(100),
				newOutput(50),
			},
			expectedTotal: amount(1150),
		},
		{
			name: "insufficient utxos",
			unspentOutputs: []tx_input.Output{
				newOutput(100),
				newOutput(200),
			},
			targetAmount: amount(500),
			minTotalUtxo: 1,
			expected: []tx_input.Output{
				newOutput(200),
				newOutput(100),
			},
			expectedTotal: amount(300),
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			result := tx_input.FilterForMinUtxoSet(v.unspentOutputs, v.targetAmount, v.minTotalUtxo)

			// Check the number of outputs
			require.Equal(t, len(v.expected), len(result), "number of outputs")

			// Check each output value
			for i, output := range result {
				fmt.Println("expected", v.expected[i].Value.String(), "output", output.Value.String())
				require.Equal(t, v.expected[i].Value.String(), output.Value.String(),
					"output %d value", i)
			}

			// Calculate total value of result
			total := xc.NewAmountBlockchainFromUint64(0)
			for _, output := range result {
				total = total.Add(&output.Value)
			}

			// Check total value
			require.Equal(t, v.expectedTotal.Uint64(), total.Uint64(), "total value")
		})
	}
}
