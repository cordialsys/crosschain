package crosschain_test

import (
	"fmt"

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
