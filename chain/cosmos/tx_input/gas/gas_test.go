package gas_test

import (
	"fmt"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cordialsys/crosschain/chain/cosmos/tx_input/gas"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func TestParseMinGasErr(t *testing.T) {

	type testcase struct {
		RawLog        string
		Denoms        []string
		ExpectedDenom string
		Amount        uint64
		Err           string
	}
	for i, tc := range []testcase{
		{
			RawLog:        "insufficient fees; got: 0uluna required: 60000uluna: insufficient fee",
			Denoms:        []string{"uluna"},
			ExpectedDenom: "uluna",
			Amount:        60000,
		},
		{
			RawLog:        "insufficient fees; got: 0uluna required: 1uluna: insufficient fee",
			Denoms:        []string{"uluna"},
			ExpectedDenom: "uluna",
			Amount:        1,
		},
		{
			RawLog:        "insufficient fees; got: 0uluna required: 1xxx: insufficient fee",
			Denoms:        []string{"uluna", "xxx"},
			ExpectedDenom: "xxx",
			Amount:        1,
		},
		{
			// still works
			RawLog:        "insufficient fees; got: 1xxx: insufficient fee",
			Denoms:        []string{"xxx"},
			ExpectedDenom: "xxx",
			Amount:        1,
		},
		{
			// still works
			RawLog:        "got: 0uluna required: 1xxx:",
			Denoms:        []string{"xxx"},
			ExpectedDenom: "xxx",
			Amount:        1,
		},
		{
			RawLog:        "provided fee < minimum global fee (0axpla < 850000000000000000axpla). Please increase the gas price.: insufficient fee",
			Denoms:        []string{"uluna", "axpla"},
			ExpectedDenom: "axpla",
			Amount:        850000000000000000,
		},
		{
			RawLog: "different msg 123 2xpla 5luna",
			Denoms: []string{"uluna", "axpla"},
			Err:    "could not parse min gas error",
		},
	} {
		fmt.Println("test case ", i, tc.RawLog)
		coin, err := gas.ParseMinGasError(&types.TxResponse{
			RawLog: tc.RawLog,
		}, tc.Denoms)
		if tc.Err != "" {
			require.ErrorContains(t, err, tc.Err)
		} else {
			require.NoError(t, err)
			require.Equal(t, sdk.Coin{
				Denom:  tc.ExpectedDenom,
				Amount: sdkmath.NewInt(int64(tc.Amount)),
			}, coin)
		}
	}
}

func TestTotalFeeToFeePerGas(t *testing.T) {
	require.Equal(t, 0.1, gas.TotalFeeToFeePerGas("100", 1000))
	require.Equal(t, float64(10000000000)/1000, gas.TotalFeeToFeePerGas("10000000000", 1000))
	require.Equal(t, float64(100)/1000000000000, gas.TotalFeeToFeePerGas("100", 1000000000000))
}
