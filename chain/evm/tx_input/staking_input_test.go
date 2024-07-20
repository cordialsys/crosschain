package tx_input_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestStakingInputSetOwner(t *testing.T) {
	input := tx_input.NewKilnStakingInput()
	input.PublicKeys = [][]byte{
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839a"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839b"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839c"),
	}
	err := input.SetOwner("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
	require.NoError(t, err)

	require.Len(t, input.Credentials, 3)
	require.Equal(t, "010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f", hex.EncodeToString(input.Credentials[0]))
	require.Equal(t, "010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f", hex.EncodeToString(input.Credentials[1]))
	require.Equal(t, "010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f", hex.EncodeToString(input.Credentials[2]))
}

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
