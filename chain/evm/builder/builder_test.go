package builder_test

import (
	"encoding/hex"
	"testing"

	xc "github.com/cordialsys/crosschain"
	xcbuilder "github.com/cordialsys/crosschain/builder"
	"github.com/cordialsys/crosschain/chain/evm/abi/exit_request"
	"github.com/cordialsys/crosschain/chain/evm/abi/stake_batch_deposit"
	"github.com/cordialsys/crosschain/chain/evm/builder"
	"github.com/cordialsys/crosschain/chain/evm/tx"
	"github.com/cordialsys/crosschain/chain/evm/tx_input"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestNewTxBuilder(t *testing.T) {
	b, err := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	require.NoError(t, err)
	require.NotNil(t, b)
}

func TestStakingTxUsesCredential(t *testing.T) {
	input := tx_input.NewBatchDepositInput()
	input.PublicKeys = [][]byte{
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839a"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839b"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839c"),
	}
	input.Signatures = [][]byte{
		make([]byte, 96),
		make([]byte, 96),
		make([]byte, 96),
	}
	credentials := [][]byte{
		hexutil.MustDecode("0x010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
		hexutil.MustDecode("0x010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
		hexutil.MustDecode("0x010000000000000000000000273b437645ba723299d07b1bdffcf508be64771f"),
	}

	txBuilder, err := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	require.NoError(t, err)
	owner := xc.Address("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
	args, _ := xcbuilder.NewStakeArgs(xc.ETH, owner, xc.NewAmountBlockchainFromUint64(1))
	trans, err := txBuilder.Stake(args, input)
	require.NoError(t, err)

	ethTx := trans.(*tx.Tx).GetEthTx()
	data := ethTx.Data()
	expected, err := stake_batch_deposit.Serialize(xc.NewChainConfig("").Base(), input.PublicKeys, credentials, input.Signatures)
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(data))
}

func TestUnstakingTx(t *testing.T) {
	input := tx_input.NewExitRequestInput()
	input.PublicKeys = [][]byte{
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839a"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839b"),
	}

	txBuilder, _ := builder.NewTxBuilder(xc.NewChainConfig("").Base())
	owner := xc.Address("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
	human, _ := xc.NewAmountHumanReadableFromStr("64")

	args, _ := xcbuilder.NewStakeArgs(xc.ETH, owner, human.ToBlockchain(18))
	trans, err := txBuilder.Unstake(args, input)
	require.NoError(t, err)

	ethTx := trans.(*tx.Tx).GetEthTx()
	require.EqualValues(t, 0, ethTx.Value().Uint64(), "unstake should not send any eth")

	data := ethTx.Data()
	expected, err := exit_request.Serialize(input.PublicKeys)
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(data))
}
