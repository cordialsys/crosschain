package factory_test

import (
	"os"
	"path/filepath"
	"testing"

	xc "github.com/cordialsys/crosschain"
	bitcointxinput "github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	cosmostxinput "github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	solanatxinput "github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/config/constants"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func factoryAndTestChains(t *testing.T) (*factory.Factory, []*xc.ChainConfig) {
	xcf := factory.NewDefaultFactory()
	eth, ok := xcf.GetChain("ETH")
	require.True(t, ok)
	matic, ok := xcf.GetChain("MATIC")
	require.True(t, ok)
	sol, ok := xcf.GetChain("SOL")
	require.True(t, ok)
	luna, ok := xcf.GetChain("LUNA")
	require.True(t, ok)

	return xcf, []*xc.ChainConfig{eth, matic, sol, luna}
}

func TestMustAmountBlockchain(t *testing.T) {
	xcf, testChains := factoryAndTestChains(t)

	for _, asset := range testChains {
		asset := asset.GetChain()
		amount := xcf.MustAmountBlockchain(asset, "10.3")

		var expected xc.AmountBlockchain
		if asset.Decimals == 6 {
			expected = xc.NewAmountBlockchainFromUint64(10300000)
		}
		if asset.Decimals == 9 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000)
		}
		if asset.Decimals == 12 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000 * 1000)
		}
		if asset.Decimals == 18 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000 * 1000000000)
		}

		require.Equal(t, expected, amount, "Error on: "+asset.Chain)
	}
}

func TestConvertAmountToHuman(t *testing.T) {
	var amountBlockchain xc.AmountBlockchain
	xcf, testChains := factoryAndTestChains(t)

	for _, asset := range testChains {
		asset := asset.GetChain()
		if asset.Decimals == 6 {
			amountBlockchain = xc.NewAmountBlockchainFromUint64(10300000)
		}
		if asset.Decimals == 9 {
			amountBlockchain = xc.NewAmountBlockchainFromUint64(10300000000)
		}
		if asset.Decimals == 12 {
			amountBlockchain = xc.NewAmountBlockchainFromUint64(10300000000 * 1000)
		}
		if asset.Decimals == 18 {
			amountBlockchain = xc.NewAmountBlockchainFromUint64(10300000000 * 1000000000)
		}

		amount, err := xcf.ConvertAmountToHuman(asset, amountBlockchain)
		require.NoError(t, err)
		require.Equal(t, "10.3", amount.String(), "Error on: "+asset.Chain)
	}
}

func TestConvertAmountToBlockchain(t *testing.T) {
	xcf, testChains := factoryAndTestChains(t)
	amountDecimal, _ := decimal.NewFromString("10.3")
	amountHuman := xc.AmountHumanReadable(amountDecimal)

	var expected xc.AmountBlockchain
	for _, asset := range testChains {
		asset := asset.GetChain()
		amount, err := xcf.ConvertAmountToBlockchain(asset, amountHuman)

		if asset.Decimals == 6 {
			expected = xc.NewAmountBlockchainFromUint64(10300000)
		}
		if asset.Decimals == 9 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000)
		}
		if asset.Decimals == 12 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000 * 1000)
		}
		if asset.Decimals == 18 {
			expected = xc.NewAmountBlockchainFromUint64(10300000000 * 1000000000)
		}

		require.NoError(t, err)
		require.Equal(t, expected, amount, "Error on: "+asset.Chain)
	}
}

func TestConvertAmountStrToBlockchain(t *testing.T) {
	xcf, testChains := factoryAndTestChains(t)
	var expected xc.AmountBlockchain
	for _, asset := range testChains {
		asset := asset.GetChain()
		amount, err := xcf.ConvertAmountStrToBlockchain(asset, "10.3")

		if asset.Decimals == 6 {
			expected = xc.NewAmountBlockchainFromUint64(10_300_000)
		}
		if asset.Decimals == 9 {
			expected = xc.NewAmountBlockchainFromUint64(10_300_000_000)
		}
		if asset.Decimals == 12 {
			expected = xc.NewAmountBlockchainFromUint64(10_300_000_000_000)
		}
		if asset.Decimals == 18 {
			expected = xc.NewAmountBlockchainFromUint64(10_300_000_000_000_000_000)
		}

		require.NoError(t, err)
		require.Equal(t, expected, amount, "Error on: "+asset.Chain)
	}

}

func TestConvertAmountStrToBlockchainErr(t *testing.T) {
	xcf, testChains := factoryAndTestChains(t)
	for _, asset := range testChains {
		amount, err := xcf.ConvertAmountStrToBlockchain(asset, "")
		require.EqualError(t, err, "can't convert  to decimal")
		require.Equal(t, xc.NewAmountBlockchainFromUint64(0), amount)

		_, err = xcf.ConvertAmountStrToBlockchain(asset, "err")
		require.EqualError(t, err, "can't convert err to decimal: exponent is not numeric")
		require.Equal(t, xc.NewAmountBlockchainFromUint64(0), amount)
	}
}

// Config functions

func TestGetChainConfig(t *testing.T) {
	xcf := factory.NewDefaultFactory()
	chain, ok := xcf.GetChain("SOL")
	require.True(t, ok)
	require.EqualValues(t, "SOL", chain.Chain)

	chain, ok = xcf.GetChain("BTC")
	require.True(t, ok)
	require.EqualValues(t, "BTC", chain.Chain)

	chain, ok = xcf.GetChain("ETH")
	require.True(t, ok)
	require.EqualValues(t, "ETH", chain.Chain)
}

func TestConfig(t *testing.T) {
	xcf := factory.NewDefaultFactory()
	cfg := xcf.GetConfig()
	require.NotNil(t, cfg)
}

func TestConfigDefaults(t *testing.T) {
	require := require.New(t)

	tmpPath, err := os.MkdirTemp("", "xctest2")
	require.NoError(err)
	tmpConfigFile := filepath.Join(tmpPath, "config.yaml")
	err = os.WriteFile(tmpConfigFile, []byte("crosschain:\n"), os.ModePerm)
	require.NoError(err)

	// change dir to somewhere that config files are not present
	os.Setenv(constants.ConfigEnv, tmpConfigFile)
	defer os.Unsetenv(constants.ConfigEnv)
	f := factory.NewDefaultFactory()
	cfg := f.GetConfig()
	require.NotNil(cfg)

	// default should be mainnet
	require.EqualValues("mainnet", cfg.Network)

	// now if we set XC_TESTNET, it should be testnet defaults
	os.Setenv("XC_TESTNET", "1")
	defer os.Unsetenv("XC_TESTNET")
	f = factory.NewDefaultFactory()
	cfg = f.GetConfig()
	require.EqualValues("testnet", cfg.Network)

	os.RemoveAll(tmpPath)
}

func TestTxInputSerDeser(t *testing.T) {
	require := require.New(t)
	xcf := factory.NewDefaultFactory()

	// Solana
	inputSolana := solanatxinput.NewTxInput()
	inputSolana.RecentBlockHash = [32]byte{1, 2, 3}
	inputSolana.ToIsATA = true
	inputSolana.ShouldCreateATA = true
	ser, err := xcf.MarshalTxInput(inputSolana)
	require.NoError(err)

	deser, err := xcf.UnmarshalTxInput(ser)
	require.NoError(err)
	typedSolana := deser.(*solanatxinput.TxInput)
	require.NotNil(typedSolana)
	require.Equal(inputSolana, typedSolana)

	// Cosmos
	inputCosmos := cosmostxinput.NewTxInput()
	inputCosmos.LegacyFromPublicKey = []byte{1, 2, 3}
	inputCosmos.AccountNumber = 1
	inputCosmos.Sequence = 2
	inputCosmos.GasLimit = 3
	inputCosmos.GasPrice = 4.5
	inputCosmos.LegacyMemo = "memo"
	ser, err = xcf.MarshalTxInput(inputCosmos)
	require.NoError(err)

	deser, err = xcf.UnmarshalTxInput(ser)
	require.NoError(err)
	typedCosmos := deser.(*cosmostxinput.TxInput)
	require.NotNil(typedCosmos)
	expected := inputCosmos
	require.Equal(expected, typedCosmos)

	// Bitcoin
	inputBtc := bitcointxinput.NewTxInput()
	inputBtc.UnspentOutputs = []bitcointxinput.Output{
		{
			Outpoint: bitcointxinput.Outpoint{
				Index: 1,
			},
			Value: xc.NewAmountBlockchainFromUint64(100),
		},
		{
			Outpoint: bitcointxinput.Outpoint{
				Index: 2,
			},
			Value: xc.NewAmountBlockchainFromUint64(200),
		},
	}
	btcBz, err := drivers.MarshalTxInput(inputBtc)
	require.NoError(err)
	inputBtc2, err := drivers.UnmarshalTxInput(btcBz)
	require.NoError(err)

	require.Equal(inputBtc.UnspentOutputs[0].Value.String(), "100")
	require.Equal(inputBtc.UnspentOutputs[1].Value.String(), "200")
	require.EqualValues(inputBtc2.(*bitcointxinput.TxInput).UnspentOutputs[0].Outpoint.Index, 1)
	require.EqualValues(inputBtc2.(*bitcointxinput.TxInput).UnspentOutputs[1].Outpoint.Index, 2)
	require.Equal(inputBtc2.(*bitcointxinput.TxInput).UnspentOutputs[0].Value.String(), "100")
	require.Equal(inputBtc2.(*bitcointxinput.TxInput).UnspentOutputs[1].Value.String(), "200")
}
