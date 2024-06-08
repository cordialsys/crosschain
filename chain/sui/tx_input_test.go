package sui

import (
	"fmt"

	"github.com/coming-chat/go-sui/v2/types"
)

func (s *CrosschainTestSuite) TestSafeFromDoubleSpend() {
	require := s.Require()
	newInput := &TxInput{
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "01"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "02"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "03"), "", 0, 0),
		},
	}
	oldInput1 := &TxInput{
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "02"), "", 0, 0), // conflict
			suiCoin(fmt.Sprintf("%064s", "04"), "", 0, 0),
		},
	}
	oldInput2_bad := &TxInput{
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "11"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "21"), "", 0, 0),
			suiCoin(fmt.Sprintf("%064s", "31"), "", 0, 0),
		},
	}
	oldInput3_bad := &TxInput{
		Coins: []*types.Coin{
			suiCoin(fmt.Sprintf("%064s", "04"), "", 0, 0),
		},
	}

	require.True(newInput.SafeFromDoubleSend(oldInput1))
	require.False(newInput.SafeFromDoubleSend(oldInput2_bad))
	require.False(newInput.SafeFromDoubleSend(oldInput3_bad))

	require.False(newInput.IndependentOf(oldInput1))
	require.True(newInput.IndependentOf(oldInput3_bad))
}
