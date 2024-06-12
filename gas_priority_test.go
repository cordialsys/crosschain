package crosschain_test

import (
	"fmt"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPriority(t *testing.T) {
	require.True(t, true)

	type testcase struct {
		input   string
		custom  bool
		decimal decimal.Decimal
		err     string
		valid   bool
	}
	vectors := []testcase{
		{
			input:  "low",
			custom: false,
			valid:  true,
		},
		{
			input:  "market",
			custom: false,
			valid:  true,
		},
		{
			input:  "aggressive",
			custom: false,
			valid:  true,
		},
		{
			input:  "very-aggressive",
			custom: false,
			valid:  true,
		},
		{
			input:   "random",
			custom:  true,
			decimal: decimal.Decimal{},
			err:     "invalid",
			valid:   false,
		},
		{
			input:   "1.2",
			custom:  true,
			decimal: decimal.NewFromFloat(1.2),
			err:     "",
			valid:   true,
		},
		{
			input:   "1.2.3",
			custom:  true,
			decimal: decimal.Decimal{},
			err:     "invalid",
			valid:   false,
		},
		{
			input:   "11.0",
			custom:  true,
			decimal: decimal.NewFromFloat(11.0),
			err:     "exceeds custom multiplier",
			valid:   false,
		},
	}

	for i, v := range vectors {
		fmt.Println("== testcase", i)
		priority := xc.GasFeePriority(v.input)

		require.Equal(t, v.custom, !priority.IsEnum())

		if !priority.IsEnum() {
			dec, err := priority.AsCustom()
			if v.err != "" {
				require.ErrorContains(t, err, v.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, v.decimal.String(), dec.String())
		}

		_, valid := xc.NewPriority(v.input)
		if v.valid {
			require.NoError(t, valid)
		} else {
			require.Error(t, valid)
		}

	}
}
