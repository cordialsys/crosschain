package crosschain_test

import (
	"fmt"
	"slices"
	"testing"

	. "github.com/cordialsys/crosschain"
	"github.com/cordialsys/crosschain/address"
	"github.com/cordialsys/crosschain/chain/eos"
	"github.com/cordialsys/crosschain/chain/evm"
	"github.com/cordialsys/crosschain/chain/kaspa"
	"github.com/cordialsys/crosschain/chain/substrate"
	"github.com/cordialsys/crosschain/factory"
	"github.com/cordialsys/crosschain/normalize"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

// Chain specific validation of the chain configs
func TestChains(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				switch chain.Driver {
				case DriverEVM, DriverEVMLegacy:
					evm.Validate(t, chain)
				case DriverSubstrate:
					substrate.Validate(t, chain)
				case DriverKaspa:
					kaspa.Validate(t, chain)
				case DriverEOS:
					eos.Validate(t, chain)
				case DriverTron:
					// pass
				case DriverTon:
					// pass
				case DriverSui:
					// pass
				case DriverDusk:
					// pass
				case DriverAptos:
					// pass
				case DriverSolana:
					// pass
				case DriverCosmos, DriverCosmosEvmos:
					// pass
				case DriverCardano, DriverFilecoin, DriverXlm, DriverXrp:
					// pass
				case DriverBitcoin, DriverBitcoinCash, DriverBitcoinLegacy:
					// pass
				case DriverInternetComputerProtocol:
					// pass
				case "":
					require.Fail(t, "unknown driver", chain.Driver)
				default:
					require.Fail(t, fmt.Sprintf("missing .Validate() for %s driver", chain.Driver))
				}
			})
		}
	}
}

func (s *CrosschainTestSuite) TestTypesAssetVsNativeAsset() {
	require := s.Require()
	require.Equal(NativeAsset("SOL"), SOL)
	require.NotEqual("SOL", SOL)
}

func (s *CrosschainTestSuite) TestAssetDriver() {
	require := s.Require()
	require.Equal(DriverBitcoin, NativeAsset(BTC).Driver())
	require.Equal(DriverEVM, NativeAsset(ETH).Driver())
	require.Equal(DriverEVMLegacy, NativeAsset(FTM).Driver())
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
		require.NotEmpty(driver.SignatureAlgorithms(), "driver is not valid")
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

func TestFeeLimitConfigured(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				require := require.New(t)
				chain := chain.GetChain()
				if chain.FeeLimit.Decimal().IsZero() {
					if len(chain.NativeAssets) == 0 {
						require.Fail(
							"Max fee is required, or additional native assets must be configured (e.g. Noble chain)",
						)
					}
					for _, na := range chain.NativeAssets {
						_, err := decimal.NewFromString(na.FeeLimit.String())
						require.NoError(err, fmt.Sprintf("%s additional asset %s (%s) max fee should be a valid decimal", chain.Chain, na.AssetId, xcf.Config.Network))
						f, _ := na.FeeLimit.Decimal().Float64()
						// paranoid non-zero check to account for floating point error
						require.True(f > 0.000000001, fmt.Sprintf("%s additional asset %s (%s) max fee should be non-zero", chain.Chain, na.AssetId, xcf.Config.Network))
					}
				} else {
					_, err := decimal.NewFromString(chain.FeeLimit.String())
					require.NoError(err, "max fee is required and should be a valid decimal")

					f, _ := chain.FeeLimit.Decimal().Float64()
					// paranoid non-zero check to account for floating point error
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
					require.Greater(len(chain.NativeAssets), 0, fmt.Sprintf("%s should have additional-native-assets (for paying fees) if no new native asset is configured", chain.Chain))
				} else {
					if slices.Contains(valid0DecimalAssets, string(chain.Chain)) {
						// valid 0-decimal native asset
					} else {
						require.NotZero(chain.Decimals, fmt.Sprintf("%s should have decimals set", chain.Chain))
					}
				}
				require.GreaterOrEqual(int(chain.Decimals), 0, fmt.Sprintf("%s should have positive decimals (%d)", chain.Chain, chain.Decimals))

				for _, na := range chain.NativeAssets {
					require.NotEmpty(na.AssetId, fmt.Sprintf("%s additional asset %s should have an asset id", chain.Chain, na.AssetId))

					require.True(
						na.ContractId != "" || na.BridgedAsset != "",
						fmt.Sprintf("%s additional asset %s must have at least one of contract_id or bridged_asset set", chain.Chain, na.AssetId),
					)

					normalizedAssetId := normalize.NormalizeAddressString(string(na.AssetId), chain.Chain)
					require.Equal(
						normalizedAssetId, string(na.AssetId),
						fmt.Sprintf("%s additional asset-id '%s' is not in a normalized format", chain.Chain, na.AssetId),
					)

					normalizedContractId := normalize.NormalizeAddressString(string(na.ContractId), chain.Chain)
					require.Equal(
						normalizedContractId, string(na.ContractId),
						fmt.Sprintf("%s additional contract-id '%s' is not in a normalized format", chain.Chain, na.ContractId),
					)

					if slices.Contains(valid0DecimalAssets, string(na.AssetId)) {
						// valid 0-decimal native asset
					} else {
						require.NotZero(na.Decimals, fmt.Sprintf("%s additional asset %s should have decimals set", chain.Chain, na.AssetId))
					}
					require.GreaterOrEqual(int(na.Decimals), 0, fmt.Sprintf("%s additional asset %s should have positive decimals", chain.Chain, na.AssetId))
					require.NotEmpty(na.FeeLimit, fmt.Sprintf("%s additional asset %s should have a max fee", chain.Chain, na.AssetId))
				}

				// AssetId and ContractId should be unique
				for i, na1 := range chain.NativeAssets {
					for j, na2 := range chain.NativeAssets {
						if i == j {
							continue
						}
						require.NotEqual(na1.AssetId, na2.AssetId, fmt.Sprintf("%s additional asset %s and %s should have unique asset ids", chain.Chain, na1.AssetId, na2.AssetId))
						require.NotEqual(na1.ContractId, na2.ContractId, fmt.Sprintf("%s additional asset %s and %s should have unique contract ids", chain.Chain, na1.AssetId, na2.AssetId))
					}
				}
			})
		}
	}
}

func TestLegacyChainCoinConfig(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				require := require.New(t)
				if chain.ChainCoin != "" {
					// there should be a native-asset-config with the corresponding chain_coin as the contract_id
					m := map[ContractAddress]*AdditionalNativeAsset{}
					for _, na := range chain.NativeAssets {
						m[na.ContractId] = na
					}
					_, ok := m[ContractAddress(chain.ChainCoin)]
					require.True(ok, fmt.Sprintf(
						"%s should have a native-asset-config with the corresponding chain_coin '%s' as the contract_id",
						chain.Chain,
						chain.ChainCoin,
					))

					na := m[ContractAddress(chain.ChainCoin)]
					if na.AssetId == string(chain.Chain) {
						require.Equal(na.Decimals, chain.Decimals, fmt.Sprintf(
							"chain %s decimals does not match native asset %s decimals",
							chain.Chain, na.AssetId,
						))
					}
				}
			})
		}
	}
}

func TestSupportedAddressFormats(t *testing.T) {
	xcf1 := factory.NewDefaultFactory()
	xcf2 := factory.NewNotMainnetsFactory(&factory.FactoryOptions{})
	for _, xcf := range []*factory.Factory{xcf1, xcf2} {
		for _, chain := range xcf.GetAllChains() {
			t.Run(fmt.Sprintf("%s_%s", chain.Chain, xcf.Config.Network), func(t *testing.T) {
				require := require.New(t)
				if len(chain.Address.Formats) > 1 {
					for _, format := range chain.Address.Formats {
						addressArgs := []address.AddressOption{}
						addressArgs = append(addressArgs, address.OptionFormat(format))
						builder, err := factory.NewDefaultFactory().NewAddressBuilder(
							chain.ChainBaseConfig, addressArgs...,
						)
						require.NoError(err)

						withFormats, ok := builder.(AddressBuilderWithFormats)
						if !ok {
							require.Fail(
								"missing AddressBuilderWithFormats",
								"address builder %T does not implement AddressBuilderWithFormats despite having multiple formats", builder,
							)
						}
						alg := withFormats.GetSignatureAlgorithm()
						require.NotEmpty(alg, "address builder %T should have a signature algorithm for format %s", builder, format)
					}

				} else if len(chain.Address.Formats) == 1 {
					require.Fail(
						"invalid address formats configuration",
						"%s, unnecessary base format", chain.Chain,
					)
				}
			})
		}
	}
}
