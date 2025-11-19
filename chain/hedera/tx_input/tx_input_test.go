package tx_input_test

import (
	"testing"
	"time"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/hedera/tx_input"
	"github.com/stretchr/testify/require"
)

type TxInput = tx_input.TxInput

func TestTxInputConflicts(t *testing.T) {

	type testcase struct {
		name     string
		oldInput xc.TxInput
		newInput xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}
	baseTs := int64(1762944396821138057)
	baseExpiration := time.Second * 180
	vectors := []testcase{
		{
			name: "SafeInputs",
			oldInput: &TxInput{
				ValidStartTimestamp: baseTs,
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			newInput: &TxInput{
				ValidStartTimestamp: baseTs + baseExpiration.Nanoseconds()*tx_input.TIME_MARGIN_MULTIPLIER + 1, // over expiration date
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			independent:     true,
			doubleSpendSafe: true,
		},
		{
			name: "ValidButNoMargin",
			oldInput: &TxInput{
				ValidStartTimestamp: baseTs,
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			newInput: &TxInput{
				ValidStartTimestamp: baseTs + baseExpiration.Nanoseconds() + 1, // over expiration date, but within margin
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name: "UnsafeInputs",
			oldInput: &TxInput{
				ValidStartTimestamp: baseTs,
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			newInput: &TxInput{
				ValidStartTimestamp: baseTs + baseExpiration.Nanoseconds() - 5, // in expiration window
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			name:     "MissingOldInput",
			oldInput: nil,
			newInput: &TxInput{
				ValidStartTimestamp: baseTs,
				ValidTime:           180,
			},
			independent:     true,
			doubleSpendSafe: true,
		},
	}
	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			require.Equal(t,
				v.independent,
				v.newInput.IndependentOf(v.oldInput),
				"IndependentOf",
			)
			require.Equal(t,
				v.doubleSpendSafe,
				v.newInput.SafeFromDoubleSend(v.oldInput),
				"SafeFromDoubleSend",
			)
		})
	}
}

func TestSetGasFeePriority(t *testing.T) {
	v := []struct {
		name        string
		baseFee     uint64
		multiplier  xc.GasFeePriority
		expectedFee uint64
	}{
		{
			name:        "Low",
			baseFee:     100_000,
			multiplier:  MustNewPriority("0.7"),
			expectedFee: 70_000,
		},
		{
			name:        "Market",
			baseFee:     100_000,
			multiplier:  MustNewPriority("1.0"),
			expectedFee: 100_000,
		},
		{
			name:        "Aggressive",
			baseFee:     100_000,
			multiplier:  MustNewPriority("1.5"),
			expectedFee: 150_000,
		},
		{
			name:        "VeryAggressive",
			baseFee:     100_000,
			multiplier:  MustNewPriority("2.0"),
			expectedFee: 200_000,
		},
	}
	for _, v := range v {
		t.Run(v.name, func(t *testing.T) {
			input := &TxInput{
				MaxTransactionFee: v.baseFee,
			}
			err := input.SetGasFeePriority(v.multiplier)
			require.NoError(t, err)
			require.Equal(t, v.expectedFee, input.MaxTransactionFee)
		})
	}
}

func MustNewPriority(p string) xc.GasFeePriority {
	feePriority, err := xc.NewPriority(p)
	if err != nil {
		panic(err)
	}
	return feePriority
}
