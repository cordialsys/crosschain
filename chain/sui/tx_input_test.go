package sui

import (
	"fmt"

	"github.com/coming-chat/go-sui/v2/types"
)

func (s *CrosschainTestSuite) TestCoinEqual() {
	require := s.Require()
	require.True(
		CoinEqual(
			suiCoin(fmt.Sprintf("%064s", "01"), "01", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "01"), "01", 0, 0),
		),
	)

	require.False(
		CoinEqual(
			suiCoin(fmt.Sprintf("%064s", "01"), "01", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "02"), "01", 0, 0),
		),
	)
	require.False(
		CoinEqual(
			suiCoin(fmt.Sprintf("%064s", "01"), "01", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "01"), "02", 0, 0),
		),
	)
}
func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "100"), "", 0, 0),
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "01"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "02"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "03"), "", 0, 0),
		},
	}
	oldInput1_confict := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "50"), "", 0, 0),
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "02"), "", 0, 0), // conflict
			suiCoin(fmt.Sprintf("%064s", "04"), "", 0, 0),
		},
	}
	oldInput2_good := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "51"), "", 0, 0),
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "11"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "21"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "31"), "", 0, 0),
		},
	}
	oldInput3_good := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "52"), "", 0, 0),
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "04"), "", 0, 0),
		},
	}
	oldInput_badGasCoin1 := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "53"), "", 0, 0),
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "100"), "", 0, 0), // conflict
		},
	}
	oldInput_badGasCoin2 := &TxInput{
		GasCoin: *suiCoin(fmt.Sprintf("%064s", "100"), "", 0, 0), // conflict
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "54"), "", 0, 0),
		},
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1_confict))
	require.False(newInput.SafeFromDoubleSend(oldInput2_good))
	require.False(newInput.SafeFromDoubleSend(oldInput3_good))

	require.False(newInput.IndependentOf(oldInput1_confict))
	require.True(newInput.IndependentOf(oldInput2_good))
	require.True(newInput.IndependentOf(oldInput3_good))

	require.False(newInput.IndependentOf(oldInput_badGasCoin1))
	require.False(newInput.IndependentOf(oldInput_badGasCoin2))
}
