package sui_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcd/btcutil/base58"
	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/sui"
	"github.com/cordialsys/go-sui-sdk/v2/types"
)

func (s *CrosschainTestSuite) TestCoinEqual() {
	require := s.Require()
	require.True(
		sui.CoinEqual(
			newPoint("01", 1),
			newPoint("01", 1),
		),
	)

	require.False(
		sui.CoinEqual(
			newPoint("01", 1),
			newPoint("02", 1),
		),
	)
	require.False(
		sui.CoinEqual(
			newPoint("01", 1),
			newPoint("01", 2),
		),
	)
}

func newPoint(hexDigest string, globalId byte) *types.Coin {
	hexDigest = fmt.Sprintf("%02s", hexDigest)
	digest, err := hex.DecodeString(hexDigest)
	if err != nil {
		panic(err)
	}

	return suiCoin(
		// 32 byte hex, 0 padded
		fmt.Sprintf("%064s", hex.EncodeToString([]byte{(globalId)})),
		base58.Encode(digest),
		0, 0,
	)
}

func (s *CrosschainTestSuite) TestTxInputConflicts() {
	require := s.Require()
	type testcase struct {
		newInput      xc.TxInput
		oldInput      xc.TxInput
		moreOldInputs []xc.TxInput

		independent     bool
		doubleSpendSafe bool
	}

	vectors := []testcase{
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 101),
				Coins: []*types.Coin{
					newPoint("00", 11),
					newPoint("00", 21),
				},
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 101),
				Coins: []*types.Coin{
					newPoint("00", 1), //conflict
					newPoint("00", 21),
				},
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 101),
				Coins: []*types.Coin{
					newPoint("01", 1), // no conflict
					newPoint("00", 21),
				},
			},
			independent:     true,
			doubleSpendSafe: false,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100), //conflict
				Coins: []*types.Coin{
					newPoint("00", 11),
					newPoint("00", 21),
				},
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 200),
				Coins: []*types.Coin{
					newPoint("00", 100), //conflict
					newPoint("00", 21),
				},
			},
			independent:     false,
			doubleSpendSafe: true,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			// conflict
			oldInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			moreOldInputs: []xc.TxInput{
				// conflict
				&sui.TxInput{
					GasCoin: *newPoint("00", 100),
					Coins: []*types.Coin{
						newPoint("00", 1),
						newPoint("00", 2),
					},
				},
				// no conflict
				&sui.TxInput{
					GasCoin: *newPoint("00", 200),
					Coins: []*types.Coin{
						newPoint("00", 10),
						newPoint("00", 11),
					},
				},
			},
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
					newPoint("00", 3),
				},
			},
			oldInput: nil,
			// must be false for both, not always independent
			independent:     false,
			doubleSpendSafe: false,
		},
		{
			// using different input types
			newInput: &sui.TxInput{
				GasCoin: *newPoint("00", 100),
				Coins: []*types.Coin{
					newPoint("00", 1),
					newPoint("00", 2),
				},
			},
			oldInput: &sui.StakingInput{
				TxInput: sui.TxInput{
					GasCoin: *newPoint("00", 100), //conflict
					Coins: []*types.Coin{
						newPoint("00", 11),
						newPoint("00", 21),
					},
				},
			},
			independent:     false,
			doubleSpendSafe: true,
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
		doubleSpendSafe := true
		allOldInputs := append(v.moreOldInputs, v.oldInput)
		for _, oldInput := range allOldInputs {
			result := v.newInput.SafeFromDoubleSend(oldInput)
			if !result {
				doubleSpendSafe = false
			}
		}
		require.Equal(
			v.doubleSpendSafe,
			doubleSpendSafe,
			"SafeFromDoubleSend",
		)
	}
}
