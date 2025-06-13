package tx_input_test

import (
	"encoding/json"
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/cordialsys/crosschain/chain/evm_legacy"
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
			oldInput:        &evm_legacy.TxInput{Nonce: 11},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &evm_legacy.TxInput{Nonce: 11},
			oldInput:        &TxInput{Nonce: 10},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			oldInput:        &TxInput{Nonce: 11},
			newInput:        &TxInput{Nonce: 12, FromAddress: xc.Address("0x123")},
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
			newInput:        &TxInput{Nonce: 10, FeePayerAddress: xc.Address("0x123"), FeePayerNonce: 10},
			oldInput:        &TxInput{Nonce: 9, FeePayerAddress: xc.Address("0x123"), FeePayerNonce: 10},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput:        &TxInput{Nonce: 10, FeePayerAddress: xc.Address("0xaaa"), FeePayerNonce: 10},
			oldInput:        &TxInput{Nonce: 9, FeePayerAddress: xc.Address("0xbbb"), FeePayerNonce: 10},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{Nonce: 10, FeePayerAddress: xc.Address("0xaaa"), FeePayerNonce: 10},
			oldInput:        &tx_input.MultiTransferInput{tx_input.TxInput{Nonce: 9, FeePayerAddress: xc.Address("0xbbb"), FeePayerNonce: 10}},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{Nonce: 10, FeePayerAddress: "", FeePayerNonce: 10},
			oldInput:        &TxInput{Nonce: 9, FeePayerAddress: "", FeePayerNonce: 9},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput:        &TxInput{FromAddress: xc.Address("0xccc"), Nonce: 10, FeePayerAddress: xc.Address("0xaaa"), FeePayerNonce: 10},
			oldInput:        &TxInput{FromAddress: xc.Address("0xddd"), Nonce: 10, FeePayerAddress: xc.Address("0xbbb"), FeePayerNonce: 10},
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
		t.Run(fmt.Sprintf("testcase %d", i), func(t *testing.T) {
			newBz, _ := json.Marshal(v.newInput)
			oldBz, _ := json.Marshal(v.oldInput)
			fmt.Printf("testcase %d - expect safe=%t, independent=%t\n     newInput = %s\n     oldInput = %s\n", i, v.doubleSpendSafe, v.independent, string(newBz), string(oldBz))
			fmt.Println()
			require.Equal(t,
				v.newInput.IndependentOf(v.oldInput),
				v.independent,
				"IndependentOf",
			)
			require.Equal(t,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				v.doubleSpendSafe,
				"SafeFromDoubleSend",
			)
		})
	}
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
			require.Error(t, err, desc)
		} else {
			require.Equal(t, v.result, uint64(v.input.GasTipCap.Uint64()), desc)
			require.Equal(t, v.result, uint64(v.input.GasPrice.Uint64()), desc)
		}
	}
}

func TestMaxFee(t *testing.T) {
	input := tx_input.NewTxInput()
	input.GasLimit = 21_000
	input.GasFeeCap = builder.GweiToWei(10)
	input.GasPrice = builder.GweiToWei(1)

	maxFee, _ := input.GetFeeLimit()
	require.EqualValues(t, builder.GweiToWei(210_000).String(), maxFee.String())

	// use gas-price if it's higher
	input.GasPrice = builder.GweiToWei(20)
	maxFee, _ = input.GetFeeLimit()
	require.EqualValues(t, builder.GweiToWei(420_000).String(), maxFee.String())
}
