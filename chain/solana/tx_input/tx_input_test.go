package tx_input

import (
	"encoding/hex"
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

// Regression test: when no fee-payer durable nonce is configured, its account
// is the zero pubkey, which equals the System Program
// (11111111111111111111111111111111) referenced by nearly every transaction.
// SetCall must not treat that spurious match as "the tx uses our durable
// nonce" and overwrite the from-address DurableNonce with the tx blockhash.
// Doing so previously made two inputs with genuinely different nonces look
// like they shared one, producing a false conflict in IndependentOf.
func TestSetCallDoesNotPatchNonceOnZeroFeePayerAccount(t *testing.T) {
	// A real transaction whose message blockhash (9pKQ...) is NOT the input's
	// configured durable nonce (81Hmv...) and which does not reference the
	// configured nonce account (4jkCK8...). The only place the zero fee-payer
	// account can match is the System Program in the account keys.
	raw := "0100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010009112d2474fe017fa4d19075e29cee436b1620b0dfee0cfb3e9615419467a16b686f3cf6e24a5ecef0e419110de68a7980aea260a7528617545dcad4b634fe92ce0a47f570eaa98fd393d987f7dd0cb87e344844d815b9d85ad4fb5cf36fb8194567a07c0f46a8370e410f5bf99110112d87c39ebd4ca091d0087e18460f14c82308b9f8a2e9eb369bc74465344d747057de1707c20d7c04398de36442eea2cc7578e098d13b3f004b629962270389482099749095cf9dc546be654b808800c6827cea5dc2c7f5c1147ee76ad644fbbbde015b52d6e31ae1bdbfe447958f19f4b06df6ced3a6c28b94986c2c8e55c0e65474f37eeae7f9087a1ed84c264b7c683a72000000000000000000000000000000000000000000000000000000000000000006a7d517192c568ee08a845f73d29788cf035c3145b21ab344d8062ea940000006ddf6e1d765a193d9cbe146ceeb79ac1cb485ed5f5b37913a8cf5857eff00a906ddf6e1ee758fde18425dbce46ccddab61afc4d83b90d27febdf928d8a18bfc17f34b976beaede1328778fe0fd0a5ce7d8cddfd565d13362ea2f53b7853160f8c97258f4e2489f1bb3d1029148e0d830b5a1399daff1084048e7bd8dbe9f859967825584cf812fec6b0593544e60c2f569c376deeb55d5f05348f9034fe6ded9b7a381c205fb8d80f44977e9697e6e4aa7af080c00f36652c3d8075a32c073bd9bba49e0fdeb3a01c108d86981ebb337e18bcf3e15fe0800e1631c9d09675d882fc98ec93da7d4d0aea71f2d35dd8de43e4aa11a67abd2888bc179138c19dd1020803060900040400000010100c0000000e0f0307050204010a0b0d0810e171c4a7a8b4287700e8764817000000"
	bz, err := hex.DecodeString(raw)
	require.NoError(t, err)
	tx, err := solana.TransactionFromBytes(bz)
	require.NoError(t, err)

	nonceAccount := solana.MustPublicKeyFromBase58("4jkCK8vpn3HdXicNLfvYWNPvBphxea3bB4YGdYvifm9A")
	nonceAuthority := solana.MustPublicKeyFromBase58("43DbAvKxhXh1oSxkJSqGosNw3HpBnmsWiak6tB5wpecN")

	newInput := &CallInput{
		TxInput: TxInput{
			DurableNonce:          solana.MustHashFromBase58("81HmvTjKHtM5xjcntqkDpnfaoLzZJJFX5Tv9UZ9bFGz4"),
			DurableNonceAccount:   nonceAccount,
			DurableNonceAuthority: nonceAuthority,
			BaseFee:               xc.NewAmountBlockchainFromUint64(5000),
			FeePayerBaseFee:       xc.NewAmountBlockchainFromUint64(5000),
		},
	}

	require.NoError(t, newInput.SetCall(NewCallPayload(tx)))

	// The tx does not actually use the configured durable nonce, so SetCall
	// should NOT have patched DurableNonce to the tx's blockhash.
	require.NotEqual(t,
		solana.MustHashFromBase58("9pKQ9Z8yQDSKDp651R5N7bag1QpGiPqu1fDPyzzf2gue"),
		newInput.DurableNonce,
		"DurableNonce must not be overwritten with the tx blockhash",
	)

	// An older transaction using the same nonce account with a different nonce
	// value must remain independent (each uses its own nonce value).
	oldInput := &TxInput{
		DurableNonce:          solana.MustHashFromBase58("9pKQ9Z8yQDSKDp651R5N7bag1QpGiPqu1fDPyzzf2gue"),
		DurableNonceAccount:   nonceAccount,
		DurableNonceAuthority: nonceAuthority,
		PrioritizationFee:     xc.NewAmountBlockchainFromUint64(447930),
	}
	require.True(t, newInput.IndependentOf(oldInput), "inputs with different nonces must be independent")
}
