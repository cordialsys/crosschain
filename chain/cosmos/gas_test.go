package cosmos

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *CrosschainTestSuite) TestParseMinGasErr() {
	require := s.Require()

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
		coin, err := ParseMinGasError(&types.TxResponse{
			RawLog: tc.RawLog,
		}, tc.Denoms)
		if tc.Err != "" {
			require.ErrorContains(err, tc.Err)
		} else {
			require.NoError(err)
			require.Equal(sdk.Coin{
				Denom:  tc.ExpectedDenom,
				Amount: sdkmath.NewInt(int64(tc.Amount)),
			}, coin)
		}
	}
}

func (s *CrosschainTestSuite) TestTotalFeeToFeePerGas() {
	require := s.Require()
	require.Equal(0.1, TotalFeeToFeePerGas("100", 1000))
	require.Equal(float64(10000000000)/1000, TotalFeeToFeePerGas("10000000000", 1000))
	require.Equal(float64(100)/1000000000000, TotalFeeToFeePerGas("100", 1000000000000))
}
