package factory_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	xc "github.com/cordialsys/crosschain"
	bitcointxinput "github.com/cordialsys/crosschain/chain/bitcoin/tx_input"
	cosmostxinput "github.com/cordialsys/crosschain/chain/cosmos/tx_input"
	remoteclient "github.com/cordialsys/crosschain/chain/crosschain"
	solanatxinput "github.com/cordialsys/crosschain/chain/solana/tx_input"
	"github.com/cordialsys/crosschain/config/constants"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/factory/drivers"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/suite"
)

type CrosschainTestSuite struct {
	suite.Suite
	Factory          *factory.Factory
	TestNativeAssets []xc.NativeAsset
	TestAssetConfigs []xc.ITask
}

func (s *CrosschainTestSuite) SetupTest() {
	s.Factory = factory.NewDefaultFactory()
	// count := 0
	// s.Factory.AllAssets.Range(func(key, value any) bool {
	// 	count++
	// 	fmt.Printf("loaded asset %d: %s %v\n", count, key, value)
	// 	return true
	// })
	s.TestNativeAssets = []xc.NativeAsset{
		xc.ETH,
		xc.MATIC,
		xc.BNB,
		xc.SOL,
		// xc.ATOM,
	}
	for _, native := range s.TestNativeAssets {
		assetConfig, _ := s.Factory.GetAssetConfig("", native)
		s.TestAssetConfigs = append(s.TestAssetConfigs, assetConfig)
	}

}

func TestCrosschain(t *testing.T) {
	suite.Run(t, new(CrosschainTestSuite))
}

// NewObject functions

func (s *CrosschainTestSuite) TestNewDefaultFactory() {
	require := s.Require()
	require.NotNil(s.Factory)
}

func (s *CrosschainTestSuite) TestNewClient() {
	require := s.Require()

	chainWithXcClient := &xc.ChainConfig{
		Driver: "solana",
		URL:    "url2",
		Clients: []*xc.ClientConfig{
			{
				Driver: xc.DriverCrosschain,
				URL:    "url1",
			},
		},
	}

	// should return a xc client
	client, _ := s.Factory.NewClient(chainWithXcClient)
	require.NotNil(client)
	_, ok := client.(*remoteclient.Client)
	require.True(ok)

	// should _not_ return a xc client
	s.Factory.NoXcClients = true
	client, _ = s.Factory.NewClient(chainWithXcClient)
	require.NotNil(client)
	_, ok = client.(*remoteclient.Client)
	require.False(ok)
	s.Factory.NoXcClients = false

	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST", URL: "a url"})
	_, err := s.Factory.NewClient(asset)
	require.ErrorContains(err, "no client defined for")

	// no clients can be derived without a driver or url
	asset, _ = s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST2"})
	_, err = s.Factory.NewClient(asset)
	require.ErrorContains(err, "no clients possible")
}

func (s *CrosschainTestSuite) TestNewTxBuilder() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		builder, _ := s.Factory.NewTxBuilder(asset)
		require.NotNil(builder)
	}

	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST"})
	_, err := s.Factory.NewTxBuilder(asset)
	require.ErrorContains(err, "no tx-builder defined for")
}

func (s *CrosschainTestSuite) TestNewSigner() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		pri := "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"
		signer, err := s.Factory.NewSigner(asset.GetChain(), pri)
		require.NoError(err)
		require.NotNil(signer)
	}

	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST"})
	_, err := s.Factory.NewSigner(asset.GetChain(), "")
	require.ErrorContains(err, "unsupported signing alg")
}

func (s *CrosschainTestSuite) TestNewAddressBuilder() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		builder, err := s.Factory.NewAddressBuilder(asset)
		require.NoError(err)
		require.NotNil(builder)
	}

	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST"})
	_, err := s.Factory.NewAddressBuilder(asset)
	require.ErrorContains(err, "no address builder defined for")
}

// GetObject functions (excluding config)

func (s *CrosschainTestSuite) TestGetAddressFromPublicKey() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		address, _ := s.Factory.GetAddressFromPublicKey(asset, []byte{})
		require.NotNil(address)
	}
}

func (s *CrosschainTestSuite) TestGetAllPossibleAddressesFromPublicKey() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		addresses, _ := s.Factory.GetAllPossibleAddressesFromPublicKey(asset, []byte{})
		require.NotNil(addresses)
	}
}

// MustObject functions

func (s *CrosschainTestSuite) TestMustAmountBlockchain() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		asset := asset.GetChain()
		amount := s.Factory.MustAmountBlockchain(asset, "10.3")

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

		require.Equal(expected, amount, "Error on: "+asset.Chain)
	}
}

func (s *CrosschainTestSuite) TestMustAddress() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		asset := asset.GetChain()
		address := s.Factory.MustAddress(asset, "myaddress") // trivial impl
		require.Equal(xc.Address("myaddress"), address, "Error on: "+asset.Chain)
	}
}

// Convert functions

func (s *CrosschainTestSuite) TestConvertAmountToHuman() {
	require := s.Require()
	var amountBlockchain xc.AmountBlockchain
	for _, asset := range s.TestAssetConfigs {
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

		amount, err := s.Factory.ConvertAmountToHuman(asset, amountBlockchain)
		require.Nil(err)
		require.Equal("10.3", amount.String(), "Error on: "+asset.Chain)
	}
	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST", Decimals: 0})
	amountBlockchain = xc.NewAmountBlockchainFromUint64(103)
	amount, err := s.Factory.ConvertAmountToHuman(asset, amountBlockchain)
	require.NoError(err)
	require.EqualValues("103", amount.String())
}

func (s *CrosschainTestSuite) TestConvertAmountToBlockchain() {
	require := s.Require()
	amountDecimal, _ := decimal.NewFromString("10.3")
	amountHuman := xc.AmountHumanReadable(amountDecimal)

	var expected xc.AmountBlockchain
	for _, asset := range s.TestAssetConfigs {
		asset := asset.GetChain()
		amount, err := s.Factory.ConvertAmountToBlockchain(asset, amountHuman)

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

		require.Nil(err)
		require.Equal(expected, amount, "Error on: "+asset.Chain)
	}
}

func (s *CrosschainTestSuite) TestConvertAmountStrToBlockchain() {
	require := s.Require()
	var expected xc.AmountBlockchain
	for _, asset := range s.TestAssetConfigs {
		asset := asset.GetChain()
		amount, err := s.Factory.ConvertAmountStrToBlockchain(asset, "10.3")

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

		require.Nil(err)
		require.Equal(expected, amount, "Error on: "+asset.Chain)
	}

	asset, _ := s.Factory.PutAssetConfig(&xc.ChainConfig{Chain: "TEST", Decimals: 0})
	amount, err := s.Factory.ConvertAmountStrToBlockchain(asset, "103")
	require.NoError(err)
	require.EqualValues(103, amount.Uint64())
}

func (s *CrosschainTestSuite) TestConvertAmountStrToBlockchainErr() {
	require := s.Require()
	for _, asset := range s.TestAssetConfigs {
		amount, err := s.Factory.ConvertAmountStrToBlockchain(asset, "")
		require.EqualError(err, "can't convert  to decimal")
		require.Equal(xc.NewAmountBlockchainFromUint64(0), amount)

		_, err = s.Factory.ConvertAmountStrToBlockchain(asset, "err")
		require.EqualError(err, "can't convert err to decimal: exponent is not numeric")
		require.Equal(xc.NewAmountBlockchainFromUint64(0), amount)
	}
}

// Config functions

func (s *CrosschainTestSuite) TestEnrichAssetConfig() {
	require := s.Require()

	assetCfgI, err := s.Factory.GetAssetConfig("USDC", "SOL")
	require.NoError(err)
	assetCfg := assetCfgI.(*xc.TokenAssetConfig)
	assetCfg.GetChain().URL = ""
	assetCfgEnriched, err := s.Factory.EnrichAssetConfig(assetCfg)
	require.NoError(err)
	require.NotNil(assetCfgEnriched)
	require.Equal("USDC", assetCfgEnriched.Asset)
	// default should be crosschainapi client
	require.NotEqual("", assetCfgEnriched.GetChain().GetAllClients()[0].URL)
	require.NotEqual("", assetCfgEnriched.GetChain().Driver)

	assetCfg.GetChain().URL = ""
	assetCfg.Chain = "TEST"
	assetCfgEnriched, err = s.Factory.EnrichAssetConfig(assetCfg)
	require.EqualError(err, "unsupported native asset: TEST")
	require.NotNil(assetCfgEnriched)
	require.Equal("USDC", assetCfgEnriched.Asset)
	require.Equal("", assetCfgEnriched.GetChain().URL)
	require.Equal(xc.NativeAsset("TEST"), assetCfgEnriched.Chain)
	require.NotEqual("", assetCfgEnriched.GetChain().Driver)

	assetCfg.GetChain().URL = ""
	assetCfg.Chain = ""
	assetCfgEnriched, err = s.Factory.EnrichAssetConfig(assetCfg)
	require.EqualError(err, "unsupported native asset: (empty)")
	require.NotNil(assetCfgEnriched)
	require.Equal("USDC", assetCfgEnriched.Asset)
	require.Equal("", assetCfgEnriched.GetChain().URL)
	require.Equal(xc.NativeAsset(""), assetCfgEnriched.Chain)
	require.NotEqual(xc.NativeAsset(""), assetCfgEnriched.GetChain().Driver)
}

func (s *CrosschainTestSuite) TestGetAssetID() {
	require := s.Require()
	assetID := xc.GetAssetIDFromAsset("USDC", "SOL")
	require.Equal(xc.AssetID("USDC.SOL"), assetID)
}

func (s *CrosschainTestSuite) TestGetAssetConfig() {
	require := s.Require()
	task, err := s.Factory.GetAssetConfig("USDC", "SOL")
	token := task.(*xc.TokenAssetConfig)
	native := task.GetChain()
	require.NoError(err)
	require.NotNil(token)
	require.Equal("USDC", token.Asset)
	require.Equal(xc.SOL, native.Chain)
}

func (s *CrosschainTestSuite) TestGetAssetConfigEdgeCases() {
	require := s.Require()
	task, err := s.Factory.GetAssetConfig("", "")
	require.Error(err)
	asset := task.GetChain()
	require.NotNil(asset)
	require.Equal(xc.NativeAsset(""), asset.Chain)
	require.Equal(xc.NativeAsset(""), asset.Chain)
}

func (s *CrosschainTestSuite) TestGetTaskConfig() {
	require := s.Require()
	asset, err := s.Factory.GetTaskConfig("sol-wrap", "SOL")
	require.Nil(err)
	require.NotNil(asset)
}

func (s *CrosschainTestSuite) TestGetTaskConfigEdgeCases() {
	require := s.Require()
	singleAsset, _ := s.Factory.GetAssetConfig("USDC", "SOL")
	asset, err := s.Factory.GetTaskConfig("", "USDC.SOL")
	require.Nil(err)
	require.NotNil(singleAsset)
	require.NotNil(asset)
	require.Equal(singleAsset, asset)
}

func (s *CrosschainTestSuite) TestGetMultiAssetConfig() {
	require := s.Require()
	asset, err := s.Factory.GetMultiAssetConfig("SOL", "WSOL.SOL")
	require.Nil(err)
	require.NotNil(asset)
}

func (s *CrosschainTestSuite) TestGetMultiAssetConfigEdgeCases() {
	require := s.Require()
	singleAsset, _ := s.Factory.GetAssetConfig("USDC", "SOL")
	tasks, err := s.Factory.GetMultiAssetConfig("USDC.SOL", "")
	require.Nil(err)
	require.NotNil(singleAsset)
	require.NotNil(tasks)
	require.NotNil(tasks[0])
	require.Equal(singleAsset, tasks[0])
}

func (s *CrosschainTestSuite) TestGetAssetConfigByContract() {
	require := s.Require()
	s.Factory.PutAssetConfig(&xc.TokenAssetConfig{
		Chain:    "ETH",
		Contract: "0xB4FBF271143F4FBf7B91A5ded31805e42b2208d0",
		Asset:    "WETH",
	})
	s.Factory.PutAssetConfig(&xc.TokenAssetConfig{
		Chain:    "SOL",
		Contract: "4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDu",
		Asset:    "USDC",
	})

	assetI, err := s.Factory.GetAssetConfigByContract("0xB4FBF271143F4FBf7B91A5ded31805e42b2208d0", "ETH")
	asset := assetI.(*xc.TokenAssetConfig)
	require.Nil(err)
	require.NotNil(asset)
	require.Equal("WETH", asset.Asset)

	assetI, err = s.Factory.GetAssetConfigByContract("4zMMC9srt5Ri5X14GAgXhaHii3GnPAEERYPJgZJDncDu", "SOL")
	asset = assetI.(*xc.TokenAssetConfig)
	require.Nil(err)
	require.NotNil(asset)
	require.Equal("USDC", asset.Asset)

	assetI, err = s.Factory.GetAssetConfigByContract("0x123456", "ETH")
	asset = assetI.(*xc.TokenAssetConfig)
	require.EqualError(err, "unknown contract: '0x123456'")
	require.NotNil(asset)
	require.Equal("", asset.Asset)
}

func (s *CrosschainTestSuite) TestPutAssetConfig() {
	require := s.Require()
	assetName := "TEST"

	assetI, err := s.Factory.GetAssetConfig(assetName, "")
	require.EqualError(err, "could not lookup asset: 'TEST.ETH'")
	require.NotNil(assetI)

	assetI, err = s.Factory.PutAssetConfig(&xc.TokenAssetConfig{Asset: assetName, Chain: "ETH"})
	fmt.Println(assetI)
	asset := assetI.(*xc.TokenAssetConfig)
	require.Nil(err)
	require.Equal(assetName, asset.Asset)

	assetI, err = s.Factory.GetAssetConfig("TEST", "")
	asset = assetI.(*xc.TokenAssetConfig)
	require.Nil(err)
	require.Equal(assetName, asset.Asset)
}

func (s *CrosschainTestSuite) TestConfig() {
	require := s.Require()
	cfg := s.Factory.GetConfig()
	require.NotNil(cfg)
}

func (s *CrosschainTestSuite) TestConfigDefaults() {
	require := s.Require()

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

func (s *CrosschainTestSuite) TestTxInputSerDeser() {
	require := s.Require()

	// Solana
	inputSolana := solanatxinput.NewTxInput()
	inputSolana.RecentBlockHash = [32]byte{1, 2, 3}
	inputSolana.ToIsATA = true
	inputSolana.ShouldCreateATA = true
	ser, err := s.Factory.MarshalTxInput(inputSolana)
	require.NoError(err)

	deser, err := s.Factory.UnmarshalTxInput(ser)
	require.NoError(err)
	typedSolana := deser.(*solanatxinput.TxInput)
	require.NotNil(typedSolana)
	require.Equal(inputSolana, typedSolana)

	// Cosmos
	inputCosmos := cosmostxinput.NewTxInput()
	inputCosmos.FromPublicKey = []byte{1, 2, 3}
	inputCosmos.AccountNumber = 1
	inputCosmos.Sequence = 2
	inputCosmos.GasLimit = 3
	inputCosmos.GasPrice = 4.5
	inputCosmos.Memo = "memo"
	ser, err = s.Factory.MarshalTxInput(inputCosmos)
	require.NoError(err)

	deser, err = s.Factory.UnmarshalTxInput(ser)
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
