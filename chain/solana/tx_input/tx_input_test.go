package tx_input

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/require"
)

func TestTxInputConflicts(t *testing.T) {
	type testcase struct {
		newInput xc.TxInput
		oldInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	startTime := int64((100 * time.Hour).Seconds())
	vectors := []testcase{
		{
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{2}),
				Timestamp:       startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{3}),
				Timestamp:       startTime - int64(SafetyTimeoutMargin.Seconds()/2),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{4}),
				Timestamp:       startTime + int64(SafetyTimeoutMargin.Seconds()),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,
			},
			oldInput: nil,
			// solana is always independent
			independent:     true,
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

func TestTxInputGetFeeLimit(t *testing.T) {
	type testcase struct {
		name              string
		unitsConsumed     uint64
		prioritizationFee xc.AmountBlockchain
		baseFee           xc.AmountBlockchain
		expectedFee       xc.AmountBlockchain
	}

	vectors := []testcase{
		{
			name:              "zero fees",
			unitsConsumed:     0,
			prioritizationFee: xc.NewAmountBlockchainFromUint64(0),
			baseFee:           xc.NewAmountBlockchainFromUint64(0),
			expectedFee:       xc.NewAmountBlockchainFromUint64(0),
		},
		{
			name:              "with prioritization fee only",
			unitsConsumed:     0,
			prioritizationFee: xc.NewAmountBlockchainFromUint64(1000), // 1000 microlamports
			baseFee:           xc.NewAmountBlockchainFromUint64(0),
			expectedFee:       xc.NewAmountBlockchainFromUint64(1400), // 1.4M units * 1000 microlamports / 1M
		},
		{
			name:              "with base fee only",
			unitsConsumed:     0,
			prioritizationFee: xc.NewAmountBlockchainFromUint64(0),
			baseFee:           xc.NewAmountBlockchainFromUint64(5000),
			expectedFee:       xc.NewAmountBlockchainFromUint64(5000),
		},
		{
			name:              "with both fees",
			unitsConsumed:     0,
			prioritizationFee: xc.NewAmountBlockchainFromUint64(1000),
			baseFee:           xc.NewAmountBlockchainFromUint64(5000),
			expectedFee:       xc.NewAmountBlockchainFromUint64(6400), // 1400 + 5000
		},
		{
			name:              "with specific compute units",
			unitsConsumed:     500_000,
			prioritizationFee: xc.NewAmountBlockchainFromUint64(1000),
			baseFee:           xc.NewAmountBlockchainFromUint64(5000),
			expectedFee:       xc.NewAmountBlockchainFromUint64(5500), // (500K * 1000) / 1M + 5000
		},
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			input := &TxInput{
				UnitsConsumed:     v.unitsConsumed,
				PrioritizationFee: v.prioritizationFee,
				BaseFee:           v.baseFee,
			}

			fee, contract := input.GetFeeLimit()
			require.Equal(t, v.expectedFee, fee)
			require.Equal(t, xc.ContractAddress(""), contract)
		})
	}
}
