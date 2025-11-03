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
				ValidStartTimestamp: baseTs + baseExpiration.Nanoseconds() + 1, // over expiration date
				ValidTime:           int64(baseExpiration.Seconds()),
			},
			independent:     true,
			doubleSpendSafe: true,
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
