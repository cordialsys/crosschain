package crosschain_test

import (
	"fmt"
	"strings"

	. "github.com/cordialsys/crosschain"
)

func (s *CrosschainTestSuite) TestTypesAssetVsNativeAsset() {
	require := s.Require()
	require.Equal(NativeAsset("SOL"), SOL)
	require.NotEqual("SOL", SOL)
}

func (s *CrosschainTestSuite) TestAssetDriver() {
	require := s.Require()
	require.Equal(DriverBitcoin, NativeAsset(BTC).Driver())
	require.Equal(DriverEVM, NativeAsset(ETH).Driver())
	require.Equal(DriverEVMLegacy, NativeAsset(BNB).Driver())
	require.Equal(DriverAptos, NativeAsset(APTOS).Driver())
	require.Equal(DriverSolana, NativeAsset(SOL).Driver())
	require.Equal(DriverCosmos, NativeAsset(ATOM).Driver())
	require.Equal(DriverSubstrate, NativeAsset(DOT).Driver())
	require.Equal(DriverTron, NativeAsset(TRX).Driver())
	require.Equal(DriverTon, NativeAsset(TON).Driver())

	drivers := map[Driver]bool{}
	for _, driver := range SupportedDrivers {
		if _, ok := drivers[driver]; ok {
			require.Fail("duplicate driver %s", driver)
		}
		drivers[driver] = true
		// test driver is valid
		require.NotEmpty(driver.SignatureAlgorithm(), "driver is not valid")
	}
}

func (s *CrosschainTestSuite) TestStakingVariants() {
	require := s.Require()

	variants := map[StakingVariant]bool{}
	for _, variant := range SupportedStakingVariants {
		parts := strings.Split(string(variant), "/")
		require.Len(parts, 4, "variant must be in format drivers/:driver/staking/:id")
		require.Equal("drivers", parts[0])
		require.Equal("staking", parts[2])
		// test driver is valid
		require.NotEmpty(Driver(parts[1]).SignatureAlgorithm(), "driver is not valid")
		require.NotEmpty(parts[3], "missing ID")

		if _, ok := variants[variant]; ok {
			require.Fail("duplicate staking variant %s", variant)
		}
		variants[variant] = true

	}
}

func (s *CrosschainTestSuite) TestChainType() {

	require := s.Require()
	// check whole native asset list
	for _, na := range NativeAssetList {
		require.True(na.IsValid(), fmt.Sprintf("%s should have a driver", na))
	}
	require.True(NativeAsset(ETH).IsValid())
	require.True(NativeAsset(BTC).IsValid())
	require.True(NativeAsset(ArbETH).IsValid())
	require.True(NativeAsset(OptETH).IsValid())
	require.True(NativeAsset("ETH").IsValid())
	require.True(NativeAsset("BTC").IsValid())
	require.True(NativeAsset("OptETH").IsValid())

	require.False(NativeAsset("xxx").IsValid())
	require.False(NativeAsset("unknown").IsValid())
}

func (s *CrosschainTestSuite) TestAssetConfig() {
	require := s.Require()
	assetConfig := ChainConfig{
		Chain:      "myasset",
		Net:        "mynet",
		URL:        "myurl",
		Auth:       "myauth",
		Provider:   "myprovider",
		AuthSecret: "SECRET",
	}
	require.Equal("NativeAssetConfig(id=myasset asset=myasset chainId=0 driver= chainCoin= prefix= net=mynet url=myurl auth=myauth provider=myprovider)", assetConfig.String())
	require.NotContains(assetConfig.String(), "SECRET")
}

func (s *CrosschainTestSuite) TestLegacyParseAssetAndNativeAsset() {
	require := s.Require()
	var asset string
	var native NativeAsset

	asset, native = LegacyParseAssetAndNativeAsset("", "SOL")
	require.Equal("SOL", asset)
	require.Equal(SOL, native)

	asset, native = LegacyParseAssetAndNativeAsset("", "ETH")
	require.Equal("ETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("ETH", "ETH")
	require.Equal("ETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC", "SOL")
	require.Equal("USDC", asset)
	require.Equal(SOL, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC", "ETH")
	require.Equal("USDC", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC", "")
	require.Equal("USDC", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC.SOL", "")
	require.Equal("USDC", asset)
	require.Equal(SOL, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC", "TRX")
	require.Equal("USDC", asset)
	require.Equal(TRX, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC.TRX", "")
	require.Equal("USDC", asset)
	require.Equal(TRX, native)

	// WETH
	asset, native = LegacyParseAssetAndNativeAsset("WETH", "")
	require.Equal("WETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH.ETH", "")
	require.Equal("WETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH", "ETH")
	require.Equal("WETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH", "MATIC")
	require.Equal("WETH", asset)
	require.Equal(MATIC, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH.MATIC", "")
	require.Equal("WETH", asset)
	require.Equal(MATIC, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH.SOL", "")
	require.Equal("WETH", asset)
	require.Equal(SOL, native)

	asset, native = LegacyParseAssetAndNativeAsset("WETH", "SOL")
	require.Equal("WETH", asset)
	require.Equal(SOL, native)
}

func (s *CrosschainTestSuite) TestLegacyParseAssetAndNativeAssetEdgeCases() {
	require := s.Require()
	var asset string
	var native NativeAsset

	asset, native = LegacyParseAssetAndNativeAsset("", "")
	require.Equal("", asset)
	require.Equal(NativeAsset(""), native)

	asset, native = LegacyParseAssetAndNativeAsset("", "test")
	require.Equal("test", asset)
	require.Equal(NativeAsset("test"), native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC.sol", "") // invalid
	require.Equal("USDC.sol", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC.WETH", "") // invalid
	require.Equal("USDC.WETH", asset)
	require.Equal(ETH, native)

	asset, native = LegacyParseAssetAndNativeAsset("USDC.ETH.SOL", "") // invalid
	require.Equal("USDC.ETH.SOL", asset)
	require.Equal(ETH, native)
}

func (s *CrosschainTestSuite) TestGetAssetIDFromAsset() {
	require := s.Require()

	require.Equal(AssetID(""), GetAssetIDFromAsset("", ""))

	require.Equal(AssetID("SOL"), GetAssetIDFromAsset("", "SOL"))
	require.Equal(AssetID("SOL"), GetAssetIDFromAsset("SOL", ""))
	require.Equal(AssetID("SOL"), GetAssetIDFromAsset("SOL", "SOL"))
	require.Equal(AssetID("SOL"), GetAssetIDFromAsset("SOL.SOL", ""))

	require.Equal(AssetID("ETH"), GetAssetIDFromAsset("", "ETH"))
	require.Equal(AssetID("ETH"), GetAssetIDFromAsset("ETH", ""))
	require.Equal(AssetID("ETH"), GetAssetIDFromAsset("ETH", "ETH"))
	require.Equal(AssetID("ETH"), GetAssetIDFromAsset("ETH.ETH", ""))

	require.Equal(AssetID("USDC.ETH"), GetAssetIDFromAsset("USDC", ""))
	require.Equal(AssetID("USDC.ETH"), GetAssetIDFromAsset("USDC", "ETH"))
	require.Equal(AssetID("USDC.ETH"), GetAssetIDFromAsset("USDC.ETH", ""))
	require.Equal(AssetID("USDC.SOL"), GetAssetIDFromAsset("USDC", "SOL"))
	require.Equal(AssetID("USDC.SOL"), GetAssetIDFromAsset("USDC.SOL", ""))

	require.Equal(AssetID("ArbETH"), GetAssetIDFromAsset("", "ArbETH"))
	require.Equal(AssetID("WETH.ArbETH"), GetAssetIDFromAsset("WETH", "ArbETH"))
	require.Equal(AssetID("WETH.ArbETH"), GetAssetIDFromAsset("WETH.ArbETH", ""))

	require.Equal(AssetID("INJ"), GetAssetIDFromAsset("INJ", "INJ"))
	require.Equal(AssetID("INJ.ETH"), GetAssetIDFromAsset("INJ", "ETH"))
	require.Equal(AssetID("INJ.SOL"), GetAssetIDFromAsset("INJ", "SOL"))

	require.Equal(AssetID("TEST.ETH"), GetAssetIDFromAsset("TEST", ""))
}
