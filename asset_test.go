package crosschain_test

import (
	"fmt"
	"slices"
	"testing"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/factory"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
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

func TestMaxFeeConfigured(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				require := require.New(t)
				chain := chain.GetChain()
				if chain.MaxFee.Decimal().IsZero() {
					if len(chain.AdditionalNativeAssets) == 0 {
						require.Fail(
							"Max fee is required, or additional native assets must be configured (e.g. Noble chain)",
						)
					}
					for _, na := range chain.AdditionalNativeAssets {
						_, err := decimal.NewFromString(na.MaxFee.String())
						require.NoError(err, fmt.Sprintf("%s additional asset %s (%s) max fee should be a valid decimal", chain.Chain, na.AssetId, xcf.Config.Network))
						f, _ := na.MaxFee.Decimal().Float64()
						require.True(f > 0.000000001, fmt.Sprintf("%s additional asset %s (%s) max fee should be non-zero", chain.Chain, na.AssetId, xcf.Config.Network))
					}
				} else {
					_, err := decimal.NewFromString(chain.MaxFee.String())
					require.NoError(err, "max fee is required and should be a valid decimal")

					f, _ := chain.MaxFee.Decimal().Float64()
					require.True(f > 0.000000001, "max fee should be non-zero")
				}
			})
		}
	}
}

func TestNativeAssetConfigs(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})

	// It's technically valid for an asset to have 0 decimals
	// If this comes up, let's add it manually here.  Otherwise we'd have to make it a pointer or add another config option.
	valid0DecimalAssets := []string{
		// (so far none)
	}

	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				require := require.New(t)
				chain := chain.GetChain()

				require.NotEmpty(chain.Chain, fmt.Sprintf("%s should have a chain", chain.Chain))
				require.NotEmpty(chain.Chain, fmt.Sprintf("%s should have a chain", chain.Chain))

				if chain.NoNativeAsset {
					// no decimals if no native asset
					require.Greater(len(chain.AdditionalNativeAssets), 0, fmt.Sprintf("%s should have additional-native-assets (for paying fees) if no new native asset is configured", chain.Chain))
				} else {
					if slices.Contains(valid0DecimalAssets, string(chain.Chain)) {
						// valid 0-decimal native asset
					} else {
						require.NotZero(chain.Decimals, fmt.Sprintf("%s should have decimals set", chain.Chain))
					}
				}
				require.GreaterOrEqual(int(chain.Decimals), 0, fmt.Sprintf("%s should have positive decimals (%d)", chain.Chain, chain.Decimals))

				for _, na := range chain.AdditionalNativeAssets {
					require.NotEmpty(na.AssetId, fmt.Sprintf("%s additional asset %s should have an asset id", chain.Chain, na.AssetId))
					if slices.Contains(valid0DecimalAssets, string(na.AssetId)) {
						// valid 0-decimal native asset
					} else {
						require.NotZero(na.Decimals, fmt.Sprintf("%s additional asset %s should have decimals set", chain.Chain, na.AssetId))
					}
					require.GreaterOrEqual(int(na.Decimals), 0, fmt.Sprintf("%s additional asset %s should have positive decimals", chain.Chain, na.AssetId))
					require.NotEmpty(na.MaxFee, fmt.Sprintf("%s additional asset %s should have a max fee", chain.Chain, na.AssetId))
				}
			})
		}
	}
}
