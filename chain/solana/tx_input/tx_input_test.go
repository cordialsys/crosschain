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
		{
			// using different input types
			newInput: &StakingInput{
				TxInput: TxInput{
					RecentBlockHash: solana.Hash([32]byte{1}),
					Timestamp:       startTime,
				},
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{2}),
				Timestamp:       startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			// durable nonce: same nonce account, different nonce values = INDEPENDENT but NOT safe
			// (they don't conflict with each other, but both could land = double send risk)
			newInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{1}),
				Timestamp:           startTime,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				DurableNonce:        solana.Hash([32]byte{10}),
			},
			oldInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{2}),
				Timestamp:           startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				DurableNonce:        solana.Hash([32]byte{11}),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			// durable nonce: same nonce account, SAME nonce value = NOT independent and SAFE
			// (they compete for the same nonce, only one can land)
			newInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{1}),
				Timestamp:           startTime,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				DurableNonce:        solana.Hash([32]byte{10}),
			},
			oldInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{2}),
				Timestamp:           startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				DurableNonce:        solana.Hash([32]byte{10}),
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			// durable nonce setup: both creating the same nonce account = NOT independent
			newInput: &TxInput{
				RecentBlockHash:          solana.Hash([32]byte{1}),
				Timestamp:                startTime,
				DurableNonceAccount:      solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				ShouldCreateDurableNonce: true,
			},
			oldInput: &TxInput{
				RecentBlockHash:          solana.Hash([32]byte{2}),
				Timestamp:                startTime,
				DurableNonceAccount:      solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				ShouldCreateDurableNonce: true,
			},
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			// durable nonce setup: creating different nonce accounts = independent
			newInput: &TxInput{
				RecentBlockHash:          solana.Hash([32]byte{1}),
				Timestamp:                startTime,
				DurableNonceAccount:      solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				ShouldCreateDurableNonce: true,
			},
			oldInput: &TxInput{
				RecentBlockHash:          solana.Hash([32]byte{2}),
				Timestamp:                startTime,
				DurableNonceAccount:      solana.MustPublicKeyFromBase58("11111111111111111111111111111113"),
				ShouldCreateDurableNonce: true,
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			// durable nonce: different nonce accounts = independent
			newInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{1}),
				Timestamp:           startTime,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				DurableNonce:        solana.Hash([32]byte{10}),
			},
			oldInput: &TxInput{
				RecentBlockHash:     solana.Hash([32]byte{2}),
				Timestamp:           startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
				DurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111113"),
				DurableNonce:        solana.Hash([32]byte{11}),
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			// fee-payer durable nonce: same account and nonce = NOT independent and SAFE
			newInput: &TxInput{
				RecentBlockHash:             solana.Hash([32]byte{1}),
				Timestamp:                   startTime,
				FeePayerDurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				FeePayerDurableNonce:        solana.Hash([32]byte{10}),
				DurableNonceAccount:         solana.MustPublicKeyFromBase58("11111111111111111111111111111113"),
				ShouldCreateDurableNonce:    true,
			},
			oldInput: &TxInput{
				RecentBlockHash:             solana.Hash([32]byte{2}),
				Timestamp:                   startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
				FeePayerDurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				FeePayerDurableNonce:        solana.Hash([32]byte{10}),
				DurableNonceAccount:         solana.MustPublicKeyFromBase58("11111111111111111111111111111114"),
				ShouldCreateDurableNonce:    true,
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			// fee-payer durable nonce: same account, different nonce = independent but NOT safe
			newInput: &TxInput{
				RecentBlockHash:             solana.Hash([32]byte{1}),
				Timestamp:                   startTime,
				FeePayerDurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				FeePayerDurableNonce:        solana.Hash([32]byte{10}),
			},
			oldInput: &TxInput{
				RecentBlockHash:             solana.Hash([32]byte{2}),
				Timestamp:                   startTime - int64(SafetyTimeoutMargin.Seconds()) - 1,
				FeePayerDurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				FeePayerDurableNonce:        solana.Hash([32]byte{11}),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			// fee-payer durable nonce setup: ignore the main nonce setup state when the
			// transaction serializes through the fee-payer nonce path.
			newInput: &TxInput{
				RecentBlockHash:             solana.Hash([32]byte{1}),
				Timestamp:                   startTime,
				DurableNonceAccount:         solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				ShouldCreateDurableNonce:    true,
				FeePayerDurableNonceAccount: solana.MustPublicKeyFromBase58("11111111111111111111111111111113"),
				FeePayerDurableNonce:        solana.Hash([32]byte{10}),
			},
			oldInput: &TxInput{
				RecentBlockHash:               solana.Hash([32]byte{2}),
				Timestamp:                     startTime,
				DurableNonceAccount:           solana.MustPublicKeyFromBase58("11111111111111111111111111111112"),
				ShouldCreateDurableNonce:      true,
				FeePayerDurableNonceAccount:   solana.MustPublicKeyFromBase58("11111111111111111111111111111113"),
				ShouldCreateFeePayerNonce:     true,
				FeePayerDurableNonceAuthority: solana.MustPublicKeyFromBase58("11111111111111111111111111111114"),
				FeePayerDurableNonce:          solana.Hash{},
			},
			independent:     true,
			doubleSpendSafe: false,
		},

		{
			// using same fee-payer durable nonce = not independent and safe
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,

				FeePayerDurableNonce:          solana.MustHashFromBase58("6MJ5iWWQRr5Pwu8efVWo2LhkmmtfZ64CYjaHTiDQpjjP"),
				FeePayerDurableNonceAccount:   solana.MustPublicKeyFromBase58("BNC16RAQsgnkM3o5eX3s4FHUtuaF4QaAhFCneSzuPUbR"),
				FeePayerDurableNonceAuthority: solana.MustPublicKeyFromBase58("G4FH1agHuh47YqBmHk8Pg4m6TQ4uhPNFFujjpjrcuhw5"),
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{2}),
				Timestamp:       startTime,

				FeePayerDurableNonce:          solana.MustHashFromBase58("6MJ5iWWQRr5Pwu8efVWo2LhkmmtfZ64CYjaHTiDQpjjP"),
				FeePayerDurableNonceAccount:   solana.MustPublicKeyFromBase58("BNC16RAQsgnkM3o5eX3s4FHUtuaF4QaAhFCneSzuPUbR"),
				FeePayerDurableNonceAuthority: solana.MustPublicKeyFromBase58("G4FH1agHuh47YqBmHk8Pg4m6TQ4uhPNFFujjpjrcuhw5"),
			},
			independent:     false,
			doubleSpendSafe: true,
		},

		{
			// using same fee-payer durable nonce = not independent and safe
			// (should ignore the value of the main durable nonce)
			newInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{1}),
				Timestamp:       startTime,

				DurableNonce:          solana.MustHashFromBase58("DMXsZD8LPPeUzjxFPG3erNgT6cQjYaBcVzmT1GxRupEF"),
				DurableNonceAccount:   solana.MustPublicKeyFromBase58("4QDJYFDBH6xLCoyGBqUBq3S5LmV91K39szrm7AE8aCgg"),
				DurableNonceAuthority: solana.MustPublicKeyFromBase58("Dv3NqyhkSERDafaZByHeFXWJMjURo4G8SHkjbkmHVTJs"),

				FeePayerDurableNonce:          solana.MustHashFromBase58("6MJ5iWWQRr5Pwu8efVWo2LhkmmtfZ64CYjaHTiDQpjjP"),
				FeePayerDurableNonceAccount:   solana.MustPublicKeyFromBase58("BNC16RAQsgnkM3o5eX3s4FHUtuaF4QaAhFCneSzuPUbR"),
				FeePayerDurableNonceAuthority: solana.MustPublicKeyFromBase58("G4FH1agHuh47YqBmHk8Pg4m6TQ4uhPNFFujjpjrcuhw5"),
			},
			oldInput: &TxInput{
				RecentBlockHash: solana.Hash([32]byte{2}),
				Timestamp:       startTime,

				DurableNonce:          solana.MustHashFromBase58("naHWJnt9VmL4pHBni3oBTxpMvqWU6B88phKvSn9ooEP"),
				DurableNonceAccount:   solana.MustPublicKeyFromBase58("4QDJYFDBH6xLCoyGBqUBq3S5LmV91K39szrm7AE8aCgg"),
				DurableNonceAuthority: solana.MustPublicKeyFromBase58("Dv3NqyhkSERDafaZByHeFXWJMjURo4G8SHkjbkmHVTJs"),

				FeePayerDurableNonce:          solana.MustHashFromBase58("6MJ5iWWQRr5Pwu8efVWo2LhkmmtfZ64CYjaHTiDQpjjP"),
				FeePayerDurableNonceAccount:   solana.MustPublicKeyFromBase58("BNC16RAQsgnkM3o5eX3s4FHUtuaF4QaAhFCneSzuPUbR"),
				FeePayerDurableNonceAuthority: solana.MustPublicKeyFromBase58("G4FH1agHuh47YqBmHk8Pg4m6TQ4uhPNFFujjpjrcuhw5"),
			},
			independent:     false,
			doubleSpendSafe: true,
		},
	}
	for i, v := range vectors {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
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
		})
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
