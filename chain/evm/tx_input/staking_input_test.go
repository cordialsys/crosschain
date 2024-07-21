package tx_input_test

import (
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/stretchr/testify/require"
)

func mustToBlockchain(human string) xc.AmountBlockchain {
	dec, err := xc.NewAmountHumanReadableFromStr(human)
	if err != nil {
		panic(err)
	}
	return dec.ToBlockchain(18)
}

func TestStakingAmount(t *testing.T) {
	chain := &xc.ChainConfig{Decimals: 18}

	div, err := tx_input.DivideAmount(chain, mustToBlockchain("32"))
	require.NoError(t, err)
	require.EqualValues(t, 1, div)
	div, err = tx_input.DivideAmount(chain, mustToBlockchain("96"))
	require.NoError(t, err)
	require.EqualValues(t, 3, div)

	_, err = tx_input.DivideAmount(chain, mustToBlockchain("48"))
	require.Error(t, err)

	_, err = tx_input.DivideAmount(chain, mustToBlockchain("32.00001"))
	require.Error(t, err)

	_, err = tx_input.DivideAmount(chain, mustToBlockchain("31"))
	require.Error(t, err)

	_, err = tx_input.DivideAmount(chain, mustToBlockchain("0"))
	require.Error(t, err)
}
