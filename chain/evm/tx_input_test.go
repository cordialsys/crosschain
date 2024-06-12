package evm

import (
	"encoding/json"
	"fmt"

	xc "github.com/cordialsys/crosschain"
)

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
			newInput:        &TxInput{Nonce: 10},
			oldInput:        &TxInput{Nonce: 10},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput:        &TxInput{Nonce: 10},
			oldInput:        &TxInput{Nonce: 11},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{Nonce: 10},
			oldInput:        &TxInput{Nonce: 9},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{Nonce: 10},
			oldInput: nil,
			// default false, not always independent
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

func (s *CrosschainTestSuite) TestTxInputGasMultiplier() {
	require := s.Require()
	type testcase struct {
		input      *TxInput
		multiplier string
		result     uint64
		err        bool
	}
	vectors := []testcase{
		{
			input:      &TxInput{GasTipCap: xc.NewAmountBlockchainFromUint64(100), GasPrice: xc.NewAmountBlockchainFromUint64(100)},
			multiplier: "1.5",
			result:     150,
		},
		{
			input:      &TxInput{GasTipCap: xc.NewAmountBlockchainFromUint64(100), GasPrice: xc.NewAmountBlockchainFromUint64(100)},
			multiplier: "abc",
			err:        true,
		},
	}
	for i, v := range vectors {
		desc := fmt.Sprintf("testcase %d: mult = %s", i, v.multiplier)
		err := v.input.SetGasFeePriority(xc.GasFeePriority(v.multiplier))
		if v.err {
			require.Error(err, desc)
		} else {
			require.Equal(v.result, uint64(v.input.GasTipCap.Uint64()), desc)
			require.Equal(v.result, uint64(v.input.GasPrice.Uint64()), desc)
		}
	}
}
