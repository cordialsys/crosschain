package builder_test

import (
	"encoding/json"
	"testing"

	xc "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/builder/buildertest"
	"github.com/cordialsys/crosschain/chain/substrate/builder"
	"github.com/cordialsys/crosschain/chain/substrate/tx_input"
	"github.com/stretchr/testify/require"
)

func taoAlphaInput(t *testing.T, positions []tx_input.AlphaPosition) *tx_input.TxInput {
	t.Helper()
	inputBz := `{
		"type":"substrate",
		"meta":{"calls":[
			{"name":"SubtensorModule.transfer_stake","section":7,"method":5},
			{"name":"Utility.batch_all","section":24,"method":2}
		],"signed_extensions":["CheckNonZeroSender","CheckSpecVersion","CheckTxVersion","CheckGenesis","CheckMortality","CheckNonce","CheckWeight","ChargeTransactionPayment","SubtensorSignedExtension"]},
		"genesis_hash":"0x2f0555cc76fc2840a25a6ea3b9637146806f1f44b090c175ffde2a7e5ab36c03",
		"current_hash":"0xaabbccdd00000000000000000000000000000000000000000000000000000000",
		"runtime_version":{"apis":[],"authoringVersion":0,"implName":"node-subtensor","implVersion":0,"specName":"node-subtensor","specVersion":100,"transactionVersion":1},
		"current_height":1000,
		"account_nonce":5,
		"tip":0
	}`
	input := &tx_input.TxInput{}
	err := json.Unmarshal([]byte(inputBz), input)
	require.NoError(t, err)
	input.AlphaPositions = positions
	return input
}

func TestAlphaTransferSinglePosition(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(500000000)

	input := taoAlphaInput(t, []tx_input.AlphaPosition{
		{Hotkey: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", Amount: 1000000000},
	})

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("18"))

	tx, err := b.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tx)
}

func TestAlphaTransferMultiplePositions(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	// Amount that spans both positions
	amount := xc.NewAmountBlockchainFromUint64(1500000000)

	input := taoAlphaInput(t, []tx_input.AlphaPosition{
		{Hotkey: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", Amount: 1000000000},
		{Hotkey: "5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb", Amount: 800000000},
	})

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("18"))

	// Should produce a batched transaction
	tx, err := b.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tx)
}

func TestAlphaTransferExactAmount(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(1000000000)

	input := taoAlphaInput(t, []tx_input.AlphaPosition{
		{Hotkey: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", Amount: 1000000000},
	})

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("18"))

	tx, err := b.Transfer(args, input)
	require.NoError(err)
	require.NotNil(tx)
}

func TestAlphaTransferInsufficientBalance(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(2000000000)

	input := taoAlphaInput(t, []tx_input.AlphaPosition{
		{Hotkey: "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty", Amount: 500000000},
	})

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("18"))

	_, err = b.Transfer(args, input)
	require.Error(err)
	require.Contains(err.Error(), "insufficient alpha balance")
}

func TestAlphaTransferNoPositions(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(1000000000)

	input := taoAlphaInput(t, nil)

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("18"))

	_, err = b.Transfer(args, input)
	require.Error(err)
	require.Contains(err.Error(), "no alpha positions")
}

func TestAlphaTransferInvalidContract(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.TAO).WithDecimals(9).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(1000000000)

	input := taoAlphaInput(t, nil)

	chainCfg := xc.NewChainConfig(xc.TAO).WithDecimals(9).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("invalid"))

	_, err = b.Transfer(args, input)
	require.Error(err)
	require.Contains(err.Error(), "expected a subnet netuid")
}

func TestNonTaoTokenTransferRejected(t *testing.T) {
	require := require.New(t)

	b, err := builder.NewTxBuilder(xc.NewChainConfig(xc.DOT).Base())
	require.NoError(err)

	from := xc.Address("5GL7deqCmoKpgmhq3b12DXSAu62VQ3DCqN3Z7Bet6fx9qAyb")
	to := xc.Address("5FUh5YJztrDvQe58YcDr5rDhkx1kSZcxQFu81wamrPuVyZSW")
	amount := xc.NewAmountBlockchainFromUint64(1000000000)

	chainCfg := xc.NewChainConfig(xc.DOT).Base()
	args := buildertest.MustNewTransferArgs(chainCfg, from, to, amount, buildertest.OptionContractAddress("some-contract"))

	input := &tx_input.TxInput{}
	_, err = b.Transfer(args, input)
	require.Error(err)
	require.Contains(err.Error(), "token transfers not supported on substrate")
}
