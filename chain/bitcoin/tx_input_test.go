package bitcoin_test

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/bitcoin"
)

func newPoint(hash []byte, index int) bitcoin.Outpoint {
	return bitcoin.Outpoint{
		Hash:  hash,
		Index: uint32(index),
	}
}
func newInput(points ...bitcoin.Outpoint) *bitcoin.TxInput {
	input := bitcoin.TxInput{}
	for _, p := range points {
		input.UnspentOutputs = append(input.UnspentOutputs, bitcoin.Output{
			Outpoint: p,
		})
	}
	return &input
}

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
