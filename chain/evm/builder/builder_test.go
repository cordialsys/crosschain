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
	b, err := builder.NewTxBuilder(&xc.TokenAssetConfig{Asset: "USDC", Contract: "1234"})
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, "USDC", b.Asset.(*xc.TokenAssetConfig).Asset)
}

func TestTransferSetsMaxTipCap(t *testing.T) {
	b, _ := builder.NewTxBuilder(&xc.ChainConfig{})

	from := "0x724435CC1B2821362c2CD425F2744Bd7347bf299"
	to := "0x3ad57b83B2E3dC5648F32e98e386935A9B10bb9F"
	amount := xc.NewAmountBlockchainFromUint64(100)
	input := tx_input.NewTxInput()

	input.GasTipCap = builder.GweiToWei(builder.DefaultMaxTipCapGwei - 1)
	trans, err := b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(t, err)
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei-1).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	input.GasTipCap = builder.GweiToWei(builder.DefaultMaxTipCapGwei + 1)
	trans, err = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.NoError(t, err)
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	// increase the max
	b, _ = builder.NewTxBuilder(&xc.ChainConfig{ChainMaxGasPrice: 100})
	trans, _ = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	// now DefaultMaxTipCapGwei + 1 is used
	require.EqualValues(t, builder.GweiToWei(builder.DefaultMaxTipCapGwei+1).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())

	// 100 is used instead of 1000
	input.GasTipCap = builder.GweiToWei(1000)
	trans, _ = b.NewTransfer(xc.Address(from), xc.Address(to), amount, input)
	require.EqualValues(t, builder.GweiToWei(100).Uint64(), trans.(*tx.Tx).EthTx.GasTipCap().Uint64())
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

	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	owner := xc.Address("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
	args, _ := xcbuilder.NewStakeArgs(xc.ETH, owner, xc.NewAmountBlockchainFromUint64(1))
	trans, err := txBuilder.Stake(args, input)
	require.NoError(t, err)

	data := trans.(*tx.Tx).EthTx.Data()
	expected, err := stake_batch_deposit.Serialize(&xc.ChainConfig{}, input.PublicKeys, credentials, input.Signatures)
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(data))
}

func TestUnstakingTx(t *testing.T) {
	input := tx_input.NewExitRequestInput()
	input.PublicKeys = [][]byte{
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839a"),
		hexutil.MustDecode("0xa776cfc875b15a1444bbda22e47e759ade11b39912a3e210807204f410d43baa332acb38aab206bc8ac7ad476a42839b"),
	}

	txBuilder, _ := builder.NewTxBuilder(&xc.ChainConfig{})
	owner := xc.Address("0x273b437645Ba723299d07B1BdFFcf508bE64771f")
	human, _ := xc.NewAmountHumanReadableFromStr("64")

	args, _ := xcbuilder.NewStakeArgs(xc.ETH, owner, human.ToBlockchain(18))
	trans, err := txBuilder.Unstake(args, input)
	require.NoError(t, err)

	require.EqualValues(t, 0, trans.(*tx.Tx).EthTx.Value().Uint64(), "unstake should not send any eth")

	data := trans.(*tx.Tx).EthTx.Data()
	expected, err := exit_request.Serialize(input.PublicKeys)
	require.NoError(t, err)

	require.Equal(t, hex.EncodeToString(expected), hex.EncodeToString(data))
}
